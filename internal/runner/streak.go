package runner

import (
	"log/slog"
	"time"

	"github.com/grubdrops/grubdrops/internal/platform"
)

// Watch streaks: Twitch adds a watch-streak bonus the first few minutes an
// account watches a broadcast. With the watch streak setting on, a Twitch
// Points watch first goes to a live force channel whose current broadcast
// the account has not yet watched for streakWatchFor, ahead of the list
// order, then goes back to the order. A drop still comes first: this only
// changes which force channel an idle account watches.

var (
	// streakWatchFor is how long a broadcast is watched for its streak. A
	// var so tests can shrink it.
	streakWatchFor = 7 * time.Minute
	// streakSeenFor is how long a served broadcast stays remembered.
	streakSeenFor = 48 * time.Hour
)

// streakOn reads the watch streak setting now. Only Twitch has streaks.
func (r *Runner) streakOn() bool {
	return r.cfg.WatchStreak != nil && r.cfg.Platform == "twitch" && r.cfg.WatchStreak()
}

// streakKey names one broadcast of login; "" when the stream has no
// broadcast ID, which then never counts as a streak candidate.
func streakKey(login string, s platform.Stream) string {
	if s.BroadcastID == "" {
		return ""
	}
	return login + "/" + s.BroadcastID
}

// streakPendingLocked reports whether key is a broadcast still owed its
// streak watch. r.mu held.
func (r *Runner) streakPendingLocked(key string) bool {
	if key == "" {
		return false
	}
	_, done := r.streakDone[key]
	return !done
}

// streakServeLocked marks the running streak watch served once it has
// lasted streakWatchFor, and forgets old broadcasts. It reports whether a
// streak watch is still running. r.mu held.
func (r *Runner) streakServeLocked(now time.Time) bool {
	if r.streakKey == "" {
		return false
	}
	if now.Sub(r.startedAt) < streakWatchFor {
		return true
	}
	r.markStreakLocked(r.streakKey, now)
	slog.Info("runner watch streak served; back to the list order", "kind", "state",
		"account", r.cfg.AccountID, "channel", r.channel)
	r.streakKey = ""
	return false
}

// markStreakLocked records broadcast key as served at now and forgets old
// broadcasts. r.mu held.
func (r *Runner) markStreakLocked(key string, now time.Time) {
	if key == "" {
		return
	}
	if r.streakDone == nil {
		r.streakDone = map[string]time.Time{}
	}
	for k, at := range r.streakDone {
		if now.Sub(at) > streakSeenFor {
			delete(r.streakDone, k)
		}
	}
	r.streakDone[key] = now
}

// streakDueLocked reports whether the running streak watch has lasted long
// enough for the force recheck to move on. r.mu held.
func (r *Runner) streakDueLocked(now time.Time) bool {
	return r.streakKey != "" && now.Sub(r.startedAt) >= streakWatchFor
}
