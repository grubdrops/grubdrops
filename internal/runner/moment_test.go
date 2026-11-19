package runner

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grubdrops/grubdrops/internal/platform"
	"github.com/grubdrops/grubdrops/internal/platform/platformtest"
)

// momentFake is a PubSubFake that can claim moments. It records moments topic
// subscriptions and claims.
type momentFake struct {
	*platformtest.PubSubFake
	mu     sync.Mutex
	subs   map[string]bool // channel ID -> moments topic subscribed
	claims []string
	events []string // "sub:<id>", "unsub:<id>", "claim:<moment>"
}

func (f *momentFake) SubscribeMoments(_ string, channelID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.subs[channelID] = true
	f.events = append(f.events, "sub:"+channelID)
}

func (f *momentFake) UnsubscribeMoments(_ string, channelID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.subs, channelID)
	f.events = append(f.events, "unsub:"+channelID)
}

func (f *momentFake) ClaimMoment(_ context.Context, _ platform.Session, momentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.claims = append(f.claims, momentID)
	f.events = append(f.events, "claim:"+momentID)
	return nil
}

func (f *momentFake) claimed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.claims...)
}

func (f *momentFake) subscribed(channelID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.subs[channelID]
}

var _ platform.MomentClaimer = (*momentFake)(nil)

func newMomentFake() *momentFake {
	return &momentFake{PubSubFake: psFake(), subs: map[string]bool{}}
}

// shrinkMomentDelay sets the claim delay for one test.
func shrinkMomentDelay(t *testing.T, d time.Duration) {
	t.Helper()
	old := momentDelay
	momentDelay = func() time.Duration { return d }
	t.Cleanup(func() { momentDelay = old })
}

// momentWatching is a runner on f drop-watching channel "a" (ID "42"), with
// claim moments read from on (nil = setting unset).
func momentWatching(t *testing.T, f *momentFake, on *atomic.Bool) *Runner {
	t.Helper()
	f.Campaigns = []platform.Campaign{camp("c1", "Rust", "d1")}
	f.Campaigns[0].Benefits[0].RequiredMinutes = 100000
	f.Progress["d1"] = &platform.Progress{BenefitID: "d1"}
	f.Streams["c1"] = []platform.Stream{{Channel: "a", ChannelID: "42", ViewerCount: 10}}
	cfg := Config{AccountID: "acc", Platform: "twitch", Backend: f, Games: rustGames, TickInterval: time.Nanosecond}
	if on != nil {
		cfg.ClaimMoments = on.Load
	}
	r := New(cfg)
	t.Cleanup(func() { r.stopWatch(context.Background()) })
	steps(t, r, 2) // pick, start
	if r.State() != StateWatching {
		t.Fatalf("state=%s", r.State())
	}
	return r
}

// With the setting unset, a watch subscribes to no moments and a pushed moment
// is not claimed.
func TestMoment_OffByDefault(t *testing.T) {
	shrinkMomentDelay(t, time.Millisecond)
	f := newMomentFake()
	r := momentWatching(t, f, nil)
	if f.subscribed("42") {
		t.Fatal("moments topic subscribed with claim moments unset")
	}
	hooksOf(t, f.PubSubFake).OnMoment("42", "moment-1")
	r.onMoment("42", "moment-2")
	time.Sleep(30 * time.Millisecond)
	if j := f.claimed(); len(j) != 0 {
		t.Fatalf("claimed %v with claim moments unset", j)
	}
}

// A moment pushed by the watched channel is claimed once, however often the
// push repeats; a moment from another channel is not claimed.
func TestMoment_ClaimsOncePerMomentID(t *testing.T) {
	shrinkMomentDelay(t, 5*time.Millisecond)
	f := newMomentFake()
	r := momentWatching(t, f, onAtomic(true))
	if !f.subscribed("42") {
		t.Fatal("drop watch did not subscribe its channel's moments")
	}
	h := hooksOf(t, f.PubSubFake)
	for range 3 {
		h.OnMoment("42", "moment-1")
	}
	h.OnMoment("99", "moment-other")
	if !waitFor(func() bool { return len(f.claimed()) >= 1 }, time.Second) {
		t.Fatal("moment not claimed")
	}
	h.OnMoment("42", "moment-1") // after the claim too
	time.Sleep(30 * time.Millisecond)
	if j := f.claimed(); !slices.Equal(j, []string{"moment-1"}) {
		t.Fatalf("claims=%v, want [moment-1]", j)
	}
	r.stopWatch(context.Background())
	if f.subscribed("42") {
		t.Fatal("moments topic kept after the watch stopped")
	}
}

// A moment whose watch stops during the delay is not claimed.
func TestMoment_NoClaimAfterWatchStops(t *testing.T) {
	shrinkMomentDelay(t, 40*time.Millisecond)
	f := newMomentFake()
	r := momentWatching(t, f, onAtomic(true))
	r.onMoment("42", "moment-1")
	r.stopWatch(context.Background())
	time.Sleep(80 * time.Millisecond)
	if j := f.claimed(); len(j) != 0 {
		t.Fatalf("claimed %v after the watch stopped", j)
	}
}

// A moment whose runner stops (run context cancelled) during the delay is not
// claimed, even before stopWatch runs.
func TestMoment_NoClaimAfterRunnerStops(t *testing.T) {
	shrinkMomentDelay(t, 40*time.Millisecond)
	f := newMomentFake()
	f.Campaigns = []platform.Campaign{camp("c1", "Rust", "d1")}
	f.Campaigns[0].Benefits[0].RequiredMinutes = 100000
	f.Progress["d1"] = &platform.Progress{BenefitID: "d1"}
	f.Streams["c1"] = []platform.Stream{{Channel: "a", ChannelID: "42", ViewerCount: 10}}
	r := New(Config{AccountID: "acc", Platform: "twitch", Backend: f, Games: rustGames,
		TickInterval: time.Nanosecond, ClaimMoments: func() bool { return true }})
	t.Cleanup(func() { r.stopWatch(context.Background()) })
	ctx, cancel := context.WithCancel(context.Background())
	for i := range 2 {
		if err := r.step(ctx); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if r.State() != StateWatching {
		t.Fatalf("state=%s", r.State())
	}
	r.onMoment("42", "moment-1")
	cancel()
	time.Sleep(80 * time.Millisecond)
	if j := f.claimed(); len(j) != 0 {
		t.Fatalf("claimed %v after the runner stopped", j)
	}
}

// After the delay, a claim goes out only while the moment task is still the
// live watch's on the announcing channel.
func TestMoment_ClaimRechecksWatch(t *testing.T) {
	f := newMomentFake()
	r := momentWatching(t, f, onAtomic(true))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stale := &momentWatch{channelID: "42", ctx: ctx, cancel: cancel}
	stale.wg.Add(1)
	r.claimMoment(stale, f, "moment-stale", 0)
	if j := f.claimed(); len(j) != 0 {
		t.Fatalf("claimed %v from a moment task the watch no longer runs", j)
	}

	r.mu.Lock()
	live := r.moments
	r.mu.Unlock()
	live.wg.Add(1)
	r.claimMoment(live, f, "moment-live", 0)
	if j := f.claimed(); !slices.Equal(j, []string{"moment-live"}) {
		t.Fatalf("claims=%v, want [moment-live]", j)
	}
}

// Turning the setting on or off applies to the running watch without
// restarting it.
func TestMoment_SettingAppliesLive(t *testing.T) {
	shrinkMomentDelay(t, time.Millisecond)
	f := newMomentFake()
	on := onAtomic(false)
	r := momentWatching(t, f, on)
	if f.subscribed("42") {
		t.Fatal("subscribed while off")
	}
	on.Store(true)
	r.applyLive(context.Background())
	if !f.subscribed("42") || r.State() != StateWatching {
		t.Fatalf("after on: subscribed=%v state=%s", f.subscribed("42"), r.State())
	}
	on.Store(false)
	r.applyLive(context.Background())
	if f.subscribed("42") || r.State() != StateWatching {
		t.Fatalf("after off: subscribed=%v state=%s", f.subscribed("42"), r.State())
	}
	r.onMoment("42", "moment-1")
	time.Sleep(20 * time.Millisecond)
	if j := f.claimed(); len(j) != 0 {
		t.Fatalf("claimed %v after turning claim moments off", j)
	}
}

// A force watch follows moments from its channel too.
func TestMoment_ForceWatchSubscribes(t *testing.T) {
	shrinkMomentDelay(t, time.Millisecond)
	f := newMomentFake()
	setLive(f.Fake, "b", true)
	cfg := Config{AccountID: "acc", Platform: "twitch", Backend: f, Games: rustGames,
		ClaimMoments: func() bool { return true }}
	forceCfg(f.Fake, "b")(&cfg)
	r := New(cfg)
	t.Cleanup(func() { r.stopWatch(context.Background()) })
	steps(t, r, 1)
	forceOn(t, r, "b")
	if !f.subscribed("id-b") {
		t.Fatal("force watch did not subscribe its channel's moments")
	}
	r.onMoment("id-b", "moment-1")
	if !waitFor(func() bool { return len(f.claimed()) == 1 }, time.Second) {
		t.Fatal("force watch moment not claimed")
	}
	forceOn(t, r, "b") // still on the original channel
}
