package runner

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/grubdrops/grubdrops/internal/platform"
)

// Second channel: Twitch credits channel points on two channels watched at
// once. With the second channel setting on, a Twitch Points watch also
// watches the next live Points watch channel after its own, in list order.
// The second watch is a side task on a watch-scoped context, like the
// points claims: it beacons on its own ticker, claims that channel's bonus
// when points are on, and moves on or stops when its channel ends. It never
// touches the primary watch, its progress or claims, and stopWatch ends it.
// Drop watches get none: Twitch credits drop progress on one channel only.
// With watch streaks on, a second watch that lasts streakWatchFor serves its
// broadcast's streak, so the primary does not switch to it for one.

var (
	// extraCheckEvery is how often the second watch rechecks which channel
	// it should watch. A var so tests can shrink it.
	extraCheckEvery = recheckInterval
	// extraBeatFloor is the shortest beacon pace of the second watch.
	extraBeatFloor = time.Second
	// extraStopTimeout bounds stopping the second watch.
	extraStopTimeout = 10 * time.Second
)

// extraTask is the running second watch of one force watch.
type extraTask struct {
	primary string
	cancel  context.CancelFunc
	done    chan struct{} // closed when the goroutine has returned
}

// stop cancels the task and waits for its goroutine, which stops its watch.
// nil-safe.
func (t *extraTask) stop() {
	if t == nil {
		return
	}
	t.cancel()
	<-t.done
}

// secondOn reads the second channel setting now. Only Twitch credits two
// channels at once.
func (r *Runner) secondOn() bool {
	return r.cfg.SecondStream != nil && r.cfg.ForceLookup != nil &&
		platform.PointsSupported(r.cfg.Platform) && r.cfg.SecondStream()
}

// startExtra starts the second watch beside the force watch on primary. It
// replaces any second watch still running. A no-op when the setting is off
// or the force watch is gone.
func (r *Runner) startExtra(ctx context.Context, primary string) {
	if !r.secondOn() || primary == "" {
		return
	}
	xctx, cancel := context.WithCancel(ctx)
	t := &extraTask{primary: primary, cancel: cancel, done: make(chan struct{})}
	r.mu.Lock()
	if r.handle == nil || r.state != StateForceWatching || r.channel != primary {
		r.mu.Unlock()
		cancel()
		return
	}
	prev := r.extra
	r.extra = t
	r.mu.Unlock()
	prev.stop()
	go r.extraLoop(xctx, t)
}

// stopExtra ends the running second watch, if any.
func (r *Runner) stopExtra() {
	r.mu.Lock()
	t := r.extra
	r.extra = nil
	r.mu.Unlock()
	t.stop()
}

// syncExtra starts or stops the second watch to match the setting now,
// keeping the force watch. Run goroutine only.
func (r *Runner) syncExtra(ctx context.Context) {
	on := r.secondOn()
	r.mu.Lock()
	running := r.extra != nil
	primary := ""
	if r.state == StateForceWatching && r.handle != nil {
		primary = r.channel
	}
	stale := running && r.extra.primary != primary
	r.mu.Unlock()
	switch {
	case on && primary != "" && (!running || stale):
		r.startExtra(ctx, primary)
	case (!on || primary == "") && running:
		r.stopExtra()
	}
}

// extraLogins is the force list without primary, in order.
func extraLogins(force []string, primary string) []string {
	out := make([]string, 0, len(force))
	for _, l := range force {
		if l != "" && !strings.EqualFold(l, primary) {
			out = append(out, l)
		}
	}
	return out
}

// extraLoop runs the second watch until ctx ends: it picks the best live
// channel after the primary, beacons it every tick, claims its bonus at the
// points cadence and rechecks the pick every extraCheckEvery.
func (r *Runner) extraLoop(ctx context.Context, t *extraTask) {
	defer close(t.done)
	var h *platform.WatchHandle
	login := ""
	release := func() {
		if h == nil {
			return
		}
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extraStopTimeout)
		if err := r.cfg.Backend.StopWatch(sctx, *h); err != nil {
			slog.Debug("runner second watch stop failed", "kind", "error", "account", r.cfg.AccountID,
				"channel", login, "err", err)
		}
		cancel()
		slog.Info("runner second watch ended", "kind", "state", "account", r.cfg.AccountID, "channel", login)
		h, login = nil, ""
		r.mu.Lock()
		r.extraChannel = ""
		r.mu.Unlock()
		r.changed()
	}
	defer release()

	every := max(r.cfg.TickInterval, extraBeatFloor)
	beat := time.NewTicker(every)
	defer beat.Stop()
	var checkedAt, claimedAt, startedAt time.Time
	var stream platform.Stream
	served := false
	for {
		now := time.Now()
		if h == nil || now.Sub(checkedAt) >= extraCheckEvery {
			checkedAt = now
			next, s, ok := r.liveForce(ctx, extraLogins(r.forceChannels(), t.primary))
			switch {
			case ctx.Err() != nil:
				return
			case !ok:
				release()
			case !strings.EqualFold(next, login):
				release()
				if s.Channel == "" {
					s.Channel = next
				}
				hh, err := r.cfg.Backend.StartWatch(ctx, r.cfg.Session, s)
				if err != nil {
					if ctx.Err() == nil {
						slog.Warn("runner second watch failed to start", "kind", "error",
							"account", r.cfg.AccountID, "channel", next, "err", err)
					}
					break
				}
				h, login, claimedAt = &hh, next, time.Time{}
				stream, startedAt, served = s, time.Now(), false
				r.mu.Lock()
				r.extraChannel = next
				r.mu.Unlock()
				r.changed()
				slog.Info("runner second watch started", "kind", "state", "account", r.cfg.AccountID,
					"channel", next, "primary", t.primary)
			}
		}
		if h != nil {
			if err := r.cfg.Backend.Heartbeat(ctx, *h); err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Warn("runner second watch heartbeat failed", "kind", "error",
					"account", r.cfg.AccountID, "channel", login, "err", err)
				release()
				checkedAt = time.Time{}
			} else {
				// Watched long enough, the second channel's broadcast has its
				// streak too: the primary will not switch to it for one.
				if !served && r.streakOn() && time.Since(startedAt) >= streakWatchFor {
					served = true
					r.mu.Lock()
					r.markStreakLocked(streakKey(login, stream), time.Now())
					r.mu.Unlock()
				}
				if pc, ok := r.pointsClaimer(); ok && time.Since(claimedAt) >= pointsEvery {
					claimedAt = time.Now()
					r.claimPoints(ctx, pc, login, pointsTimeout)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-beat.C:
		}
	}
}
