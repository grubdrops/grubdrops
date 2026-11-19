package runner

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/grubdrops/grubdrops/internal/plan"
	"github.com/grubdrops/grubdrops/internal/platform"
)

// repeatMargin is how many beacons a locally tracked repeat day waits past
// its requirement: Twitch credits a minute per beacon, but the first ones can
// land before the watch counts.
const repeatMargin = 3

// repeatTick returns progress with the current target's repeat-day row added
// when the backend reported none (Twitch's own progress read is integrity-
// gated or silent). It counts one minute per successful beacon on a stream of
// the group's category, restarting the count when the group's reset interval
// has passed since it began. Other targets and rows the backend gave pass
// through.
func (r *Runner) repeatTick(target plan.Target, stream *platform.Stream, progress []platform.Progress) []platform.Progress {
	b := target.Benefit
	if b.Repeat == nil || slices.ContainsFunc(progress, func(p platform.Progress) bool { return p.BenefitID == b.ID }) {
		return progress
	}
	now := time.Now()
	r.mu.Lock()
	if r.repeatMin == nil {
		r.repeatMin, r.repeatFirst = map[string]int{}, map[string]time.Time{}
	}
	if first, ok := r.repeatFirst[b.ID]; !ok || (b.Repeat.Reset > 0 && now.Sub(first) >= b.Repeat.Reset) {
		r.repeatFirst[b.ID] = now
		r.repeatMin[b.ID] = 0
	}
	if len(b.Repeat.CategoryIDs) == 0 || (stream != nil && slices.Contains(b.Repeat.CategoryIDs, stream.GameID)) {
		r.repeatMin[b.ID]++
	}
	n := r.repeatMin[b.ID]
	r.mu.Unlock()
	row := platform.Progress{BenefitID: b.ID, MinutesWatched: max(0, n-repeatMargin)}
	if row.MinutesWatched == b.RequiredMinutes {
		slog.Info("runner repeat day reached its minutes by local count", "kind", "progress",
			"account", r.cfg.AccountID, "benefit", b.ID, "source", "local", "min_watched", row.MinutesWatched)
	}
	return append(slices.Clone(progress), row)
}

// withRepeatLocal gates each unclaimed repeat day the backend gave no row for
// on the claim log: the earlier day's logged time plus the group's reset
// interval. Such a day gets a row when it has a gate or locally counted
// minutes. A backend row keeps Twitch's own window gate.
func (r *Runner) withRepeatLocal(ctx context.Context, camps []platform.Campaign, progress []platform.Progress, claimed map[string]bool) []platform.Progress {
	progress = slices.Clone(progress)
	idx := make(map[string]int, len(progress))
	for i, p := range progress {
		idx[p.BenefitID] = i
	}
	corrector, _ := r.cfg.Claims.(ClaimCorrector)
	for _, c := range camps {
		for _, b := range c.Benefits {
			if b.Repeat == nil || claimed[b.ID] {
				continue
			}
			var gate time.Time
			if corrector != nil && len(b.Preconditions) > 0 {
				at, _, ok, err := corrector.LoggedClaim(ctx, r.cfg.AccountID, b.Preconditions[0])
				if err == nil && ok {
					gate = at.Add(b.Repeat.Reset)
				}
			}
			if _, ok := idx[b.ID]; ok {
				// Twitch's row carries its own window gate; the log gate
				// would hold the day past Twitch's reset.
				continue
			}
			row := platform.Progress{BenefitID: b.ID, NextEligibleAt: gate}
			r.mu.Lock()
			row.MinutesWatched = max(0, r.repeatMin[b.ID]-repeatMargin)
			r.mu.Unlock()
			if row.MinutesWatched > 0 || !gate.IsZero() {
				progress = append(progress, row)
			}
		}
	}
	return progress
}

// logRepeatClaimed records in the claim log each repeat day Twitch reports
// claimed that the log lacks, so the log gate of the next day has its time.
// It returns claimed with them added, as a copy when it changes.
func (r *Runner) logRepeatClaimed(ctx context.Context, camps []platform.Campaign, progress []platform.Progress, claimed map[string]bool) map[string]bool {
	if r.cfg.Claims == nil {
		return claimed
	}
	done, copied := map[string]bool{}, false
	for _, p := range progress {
		if p.Claimed {
			done[p.BenefitID] = true
		}
	}
	for _, c := range camps {
		for _, b := range c.Benefits {
			if b.Repeat == nil || !done[b.ID] || claimed[b.ID] {
				continue
			}
			rctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := r.cfg.Claims.RecordClaim(rctx, r.cfg.AccountID, plan.Target{Campaign: c, Benefit: b, Minutes: b.RequiredMinutes}, "")
			cancel()
			if err != nil {
				slog.Warn("runner record repeat claim failed", "kind", "error", "account", r.cfg.AccountID, "benefit", b.ID, "err", err)
				continue
			}
			if !copied {
				claimed, copied = maps.Clone(claimed), true
				if claimed == nil {
					claimed = map[string]bool{}
				}
			}
			claimed[b.ID] = true
			slog.Info("runner repeat day claimed on twitch recorded", "kind", "claim", "account", r.cfg.AccountID, "benefit", b.ID)
		}
	}
	return claimed
}

// minTime returns the earlier non-zero of a and b, or zero when both are.
func minTime(a, b time.Time) time.Time {
	switch {
	case a.IsZero():
		return b
	case b.IsZero(), a.Before(b):
		return a
	}
	return b
}
