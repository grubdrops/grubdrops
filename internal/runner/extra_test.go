package runner

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grubdrops/grubdrops/internal/platform"
	"github.com/grubdrops/grubdrops/internal/platform/platformtest"
)

// shrinkExtra sets the second watch's recheck and beacon pace for one test.
func shrinkExtra(t *testing.T, check, beat time.Duration) {
	t.Helper()
	oc, ob := extraCheckEvery, extraBeatFloor
	extraCheckEvery, extraBeatFloor = check, beat
	t.Cleanup(func() { extraCheckEvery, extraBeatFloor = oc, ob })
}

// secondCfg is forceCfg on a Twitch account with the second channel setting
// read from on.
func secondCfg(f *platformtest.Fake, on *atomic.Bool, channels ...string) func(*Config) {
	force := forceCfg(f, channels...)
	return func(c *Config) {
		force(c)
		c.Platform = "twitch"
		c.TickInterval = 5 * time.Millisecond
		if on != nil {
			c.SecondStream = on.Load
		}
	}
}

func extraOf(r *Runner) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.extraChannel
}

// With the setting on, a Twitch Points watch also watches and beacons the
// next live channel of its list; stopping the watch ends both.
func TestSecond_WatchesNextLiveChannel(t *testing.T) {
	shrinkExtra(t, time.Hour, time.Millisecond)
	f := platformtest.NewFake()
	setLive(f, "a", true)
	setLive(f, "b", false)
	setLive(f, "c", true)
	r := newRunner(f, secondCfg(f, onAtomic(true), "a", "b", "c"))
	steps(t, r, 1) // pick: a
	forceOn(t, r, "a")
	if !waitFor(func() bool { return extraOf(r) == "c" && countCall(f, "Heartbeat:c") >= 2 }, time.Second) {
		t.Fatalf("second watch: extra=%q calls=%v", extraOf(r), f.Calls)
	}
	if s := r.Snapshot(); s.Channel != "a" || s.ExtraChannel != "c" {
		t.Fatalf("snapshot channel=%q extra=%q", s.Channel, s.ExtraChannel)
	}
	r.stopWatch(context.Background())
	if extraOf(r) != "" || !hasCall(f, "Stop:c") {
		t.Fatalf("after stop: extra=%q calls=%v", extraOf(r), f.Calls)
	}
}

// With the setting off or unset, only the primary channel is watched.
func TestSecond_OffWatchesOne(t *testing.T) {
	shrinkExtra(t, time.Hour, time.Millisecond)
	for _, on := range []*atomic.Bool{nil, onAtomic(false)} {
		f := platformtest.NewFake()
		setLive(f, "a", true)
		setLive(f, "b", true)
		r := newRunner(f, secondCfg(f, on, "a", "b"))
		steps(t, r, 1)
		forceOn(t, r, "a")
		time.Sleep(30 * time.Millisecond)
		if hasCall(f, "Start:b") || extraOf(r) != "" {
			t.Fatalf("second watch with the setting off: %v", f.Calls)
		}
		r.stopWatch(context.Background())
	}
}

// Kick credits one channel: no second watch there.
func TestSecond_KickWatchesOne(t *testing.T) {
	shrinkExtra(t, time.Hour, time.Millisecond)
	f := platformtest.NewFake()
	setLive(f, "a", true)
	setLive(f, "b", true)
	r := newRunner(f, func(c *Config) {
		secondCfg(f, onAtomic(true), "a", "b")(c)
		c.Platform = "kick"
	})
	t.Cleanup(func() { r.stopWatch(context.Background()) })
	steps(t, r, 1)
	forceOn(t, r, "a")
	time.Sleep(30 * time.Millisecond)
	if hasCall(f, "Start:b") {
		t.Fatalf("Kick started a second watch: %v", f.Calls)
	}
}

// A drop watch gets no second watch: drops credit one channel.
func TestSecond_DropWatchWatchesOne(t *testing.T) {
	shrinkExtra(t, time.Hour, time.Millisecond)
	f := platformtest.NewFake()
	f.Campaigns = []platform.Campaign{camp("c1", "Rust", "d1")}
	f.Campaigns[0].Benefits[0].RequiredMinutes = 100000
	f.Progress["d1"] = &platform.Progress{BenefitID: "d1"}
	f.Streams["c1"] = []platform.Stream{{Channel: "x", ViewerCount: 9}}
	setLive(f, "b", true)
	r := newRunner(f, secondCfg(f, onAtomic(true), "b"))
	t.Cleanup(func() { r.stopWatch(context.Background()) })
	steps(t, r, 2) // pick, start the drop watch on x
	if s := r.Snapshot(); s.State != StateWatching || s.Channel != "x" {
		t.Fatalf("%+v", s)
	}
	time.Sleep(30 * time.Millisecond)
	if hasCall(f, "Start:b") {
		t.Fatalf("drop watch started a second watch: %v", f.Calls)
	}
}

// When the second channel ends, the recheck stops its watch; when another
// comes live, it watches that one.
func TestSecond_FollowsLiveChannels(t *testing.T) {
	shrinkExtra(t, 10*time.Millisecond, time.Millisecond)
	f := platformtest.NewFake()
	setLive(f, "a", true)
	setLive(f, "b", true)
	setLive(f, "c", false)
	r := newRunner(f, secondCfg(f, onAtomic(true), "a", "b", "c"))
	t.Cleanup(func() { r.stopWatch(context.Background()) })
	steps(t, r, 1)
	forceOn(t, r, "a")
	if !waitFor(func() bool { return extraOf(r) == "b" }, time.Second) {
		t.Fatalf("extra=%q", extraOf(r))
	}
	setLive(f, "b", false)
	if !waitFor(func() bool { return extraOf(r) == "" && hasCall(f, "Stop:b") }, time.Second) {
		t.Fatalf("second watch kept an offline channel: extra=%q", extraOf(r))
	}
	setLive(f, "c", true)
	if !waitFor(func() bool { return extraOf(r) == "c" }, time.Second) {
		t.Fatalf("second watch did not move to c: extra=%q", extraOf(r))
	}
}

// Turning the setting on or off applies to the running Points watch.
func TestSecond_SettingAppliesLive(t *testing.T) {
	shrinkExtra(t, time.Hour, time.Millisecond)
	f := platformtest.NewFake()
	setLive(f, "a", true)
	setLive(f, "b", true)
	on := onAtomic(false)
	r := newRunner(f, secondCfg(f, on, "a", "b"))
	t.Cleanup(func() { r.stopWatch(context.Background()) })
	steps(t, r, 1)
	forceOn(t, r, "a")
	on.Store(true)
	r.applyLive(context.Background())
	if !waitFor(func() bool { return extraOf(r) == "b" }, time.Second) {
		t.Fatalf("after on: extra=%q", extraOf(r))
	}
	on.Store(false)
	r.applyLive(context.Background())
	if extraOf(r) != "" || r.State() != StateForceWatching {
		t.Fatalf("after off: extra=%q state=%s", extraOf(r), r.State())
	}
}

// With watch streaks on, a second watch long enough serves its broadcast's
// streak: once the primary's streak is served it stays in list order
// instead of switching to the second channel for its streak.
func TestSecond_ServesStreak(t *testing.T) {
	shrinkExtra(t, time.Hour, time.Millisecond)
	shrinkStreak(t, 30*time.Millisecond)
	f := platformtest.NewFake()
	setLive(f, "a", true)
	setLive(f, "b", true)
	r := newRunner(f, func(c *Config) {
		secondCfg(f, onAtomic(true), "a", "b")(c)
		c.WatchStreak = func() bool { return true }
	})
	t.Cleanup(func() { r.stopWatch(context.Background()) })
	steps(t, r, 1) // pick: a, owed its streak; b second
	forceOn(t, r, "a")
	if !waitFor(func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		_, done := r.streakDone["b/bc-b"]
		return done
	}, time.Second) {
		t.Fatal("second watch did not serve b's streak")
	}
	time.Sleep(40 * time.Millisecond)
	steps(t, r, 1) // a's streak served: recheck keeps a, b is not owed one
	forceOn(t, r, "a")
	if hasCall(f, "Start:b") && countCall(f, "Start:b") > 1 {
		t.Fatalf("primary switched to b: %v", f.Calls)
	}
}
