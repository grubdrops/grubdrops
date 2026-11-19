package runner

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/grubdrops/grubdrops/internal/platform/platformtest"
)

// shrinkStreak sets the streak watch length for one test.
func shrinkStreak(t *testing.T, d time.Duration) {
	t.Helper()
	old := streakWatchFor
	streakWatchFor = d
	t.Cleanup(func() { streakWatchFor = old })
}

// streakCfg is forceCfg on a Twitch account with watch streaks read from on.
func streakCfg(f *platformtest.Fake, on *atomic.Bool, channels ...string) func(*Config) {
	force := forceCfg(f, channels...)
	return func(c *Config) {
		force(c)
		c.Platform = "twitch"
		if on != nil {
			c.WatchStreak = on.Load
		}
	}
}

func countCall(f *platformtest.Fake, call string) int {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	n := 0
	for _, c := range f.Calls {
		if c == call {
			n++
		}
	}
	return n
}

// A new broadcast on a lower-ranked channel is watched first for its
// streak, then the watch goes back to the list order.
func TestStreak_ServesNewBroadcastThenReturns(t *testing.T) {
	shrinkStreak(t, 30*time.Millisecond)
	f := platformtest.NewFake()
	setLive(f, "a", true)
	setLive(f, "b", false)
	r := newRunner(f, streakCfg(f, onAtomic(true), "a", "b"))
	steps(t, r, 1) // pick: a, its broadcast owed a streak
	forceOn(t, r, "a")

	time.Sleep(40 * time.Millisecond)
	steps(t, r, 1) // the streak watch is served: recheck, a stays
	forceOn(t, r, "a")

	setLive(f, "b", true)
	dueRecheck(r)
	steps(t, r, 1) // b's new broadcast goes first
	forceOn(t, r, "b")

	dueRecheck(r)
	steps(t, r, 1) // still serving b's streak: keeps b
	forceOn(t, r, "b")

	time.Sleep(40 * time.Millisecond)
	steps(t, r, 1) // b served: back to a, ranked first
	forceOn(t, r, "a")
	if n := countCall(f, "Start:a"); n != 2 {
		t.Fatalf("Start:a %d times, want 2; calls=%v", n, f.Calls)
	}

	dueRecheck(r)
	steps(t, r, 1) // both served: the order holds
	forceOn(t, r, "a")
}

// With the setting off or unset, the list order alone decides.
func TestStreak_OffKeepsOrder(t *testing.T) {
	for _, on := range []*atomic.Bool{nil, onAtomic(false)} {
		f := platformtest.NewFake()
		setLive(f, "a", true)
		setLive(f, "b", false)
		r := newRunner(f, streakCfg(f, on, "a", "b"))
		steps(t, r, 1)
		forceOn(t, r, "a")
		setLive(f, "b", true)
		dueRecheck(r)
		steps(t, r, 1)
		forceOn(t, r, "a")
		if hasCall(f, "Start:b") {
			t.Fatalf("switched to b with streaks off: %v", f.Calls)
		}
	}
}

// Kick has no watch streaks: the setting changes nothing there.
func TestStreak_KickIgnored(t *testing.T) {
	f := platformtest.NewFake()
	setLive(f, "a", true)
	setLive(f, "b", false)
	r := newRunner(f, func(c *Config) {
		streakCfg(f, onAtomic(true), "a", "b")(c)
		c.Platform = "kick"
	})
	steps(t, r, 1)
	forceOn(t, r, "a")
	setLive(f, "b", true)
	dueRecheck(r)
	steps(t, r, 1)
	forceOn(t, r, "a")
}

// A streak watch whose stream ends moves on at the next recheck, like any
// force watch.
func TestStreak_EndedStreamMovesOn(t *testing.T) {
	shrinkStreak(t, time.Hour)
	f := platformtest.NewFake()
	setLive(f, "a", false)
	setLive(f, "b", true)
	r := newRunner(f, streakCfg(f, onAtomic(true), "a", "b"))
	steps(t, r, 1)
	forceOn(t, r, "b")
	setLive(f, "b", false)
	setLive(f, "a", true)
	dueRecheck(r)
	steps(t, r, 1)
	forceOn(t, r, "a")
}
