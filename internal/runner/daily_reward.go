package runner

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/grubdrops/grubdrops/internal/i18n"
	"github.com/grubdrops/grubdrops/internal/notify"
	"github.com/grubdrops/grubdrops/internal/platform"
)

// DailyRewardEvery is how often a runner checks for a daily reward to claim
// (Config.KickDailyReward). The first check runs when the runner starts.
const DailyRewardEvery = 10 * time.Minute

// dailyRewardLoop checks for a daily reward now and then every
// DailyRewardEvery until ctx ends. A check that falls in the sleep window
// waits for the window to end. It runs beside Run, so the watch beacon never
// waits on it.
func (r *Runner) dailyRewardLoop(ctx context.Context, c platform.DailyRewardClaimer) {
	for {
		for r.InSleepWindow() {
			if !sleepCtx(ctx, sleepPoll) {
				return
			}
		}
		r.claimDailyRewards(ctx, c)
		if !sleepCtx(ctx, DailyRewardEvery) {
			return
		}
	}
}

// dailyRewardOn reads the daily reward setting now.
func (r *Runner) dailyRewardOn() bool {
	return r.cfg.KickDailyReward != nil && r.cfg.KickDailyReward()
}

// claimDailyRewards claims the account's ready daily rewards while the
// setting is on. A reward whose claim failed is not tried again for the life
// of the runner, so a refused claim is not re-sent every check.
func (r *Runner) claimDailyRewards(ctx context.Context, c platform.DailyRewardClaimer) {
	if !r.dailyRewardOn() {
		return
	}
	got, err := c.ClaimDailyRewards(ctx, r.cfg.Session, r.dailyFailedID)
	for _, d := range got {
		slog.Info("runner daily reward claimed", "kind", "claim", "account", r.cfg.AccountID,
			"challenge", d.ID, "reward", d.Reward, "rarity", d.Rarity)
		r.notifyDailyReward(ctx, d)
	}
	if err == nil || ctx.Err() != nil {
		return
	}
	failed := false
	for _, e := range flatErrors(err) {
		var de *platform.DailyRewardError
		if !errors.As(e, &de) {
			continue
		}
		failed = true
		r.mu.Lock()
		if r.dailyFailed == nil {
			r.dailyFailed = map[string]bool{}
		}
		r.dailyFailed[de.ID] = true
		r.mu.Unlock()
		slog.Warn("runner daily reward claim failed; not retried this run", "kind", "claim",
			"account", r.cfg.AccountID, "challenge", de.ID, "err", de.Err)
	}
	if !failed {
		slog.Warn("runner daily reward check failed", "kind", "error", "account", r.cfg.AccountID, "err", err)
	}
}

// flatErrors splits an errors.Join result into its parts.
func flatErrors(err error) []error {
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		return j.Unwrap()
	}
	return []error{err}
}

// dailyFailedID reports whether the claim of daily reward id failed earlier
// in this run.
func (r *Runner) dailyFailedID(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dailyFailed[id]
}

// notifyDailyReward sends a claim notification for daily reward d: the
// reward and its rarity when Kick names them.
func (r *Runner) notifyDailyReward(ctx context.Context, d platform.DailyReward) {
	if r.cfg.Notifier == nil {
		return
	}
	lang := "en"
	if r.cfg.Lang != nil {
		lang = r.cfg.Lang()
	}
	title := i18n.T(lang, "notify.kick_daily_reward_claimed")
	reward := d.Reward
	if reward != "" {
		if d.Rarity != "" {
			reward += " (" + d.Rarity + ")"
		}
		title = strings.ReplaceAll(i18n.T(lang, "notify.kick_daily_reward"), "{reward}", reward)
	}
	f := r.notifyFields(map[string]any{
		"drop":                title,
		notify.FieldFeedKind:  notify.FeedKindDailyReward,
		notify.FieldFeedTitle: reward,
	})
	// Not the mined drop: drop the watch's own context.
	for _, k := range []string{"game", "campaign", "image", "channel"} {
		delete(f, k)
	}
	_ = r.cfg.Notifier.Notify(ctx, "claim", f)
}
