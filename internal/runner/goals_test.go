package runner

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grubdrops/grubdrops/internal/platform"
	"github.com/grubdrops/grubdrops/internal/platform/platformtest"
)

// goalsFake is a pointsFake that can contribute to community goals. It
// records the channels it was asked to contribute on.
type goalsFake struct {
	*pointsFake
	gmu   sync.Mutex
	goals []string
}

func (g *goalsFake) ContributeCommunityGoals(_ context.Context, _ platform.Session, channel string) (platform.GoalContribution, error) {
	g.gmu.Lock()
	defer g.gmu.Unlock()
	g.goals = append(g.goals, channel)
	return platform.GoalContribution{Goals: 1, Points: 10}, nil
}

func (g *goalsFake) contributed() []string {
	g.gmu.Lock()
	defer g.gmu.Unlock()
	return append([]string(nil), g.goals...)
}

var _ platform.GoalContributor = (*goalsFake)(nil)

func newGoalsFake() *goalsFake {
	return &goalsFake{pointsFake: &pointsFake{Fake: platformtest.NewFake()}}
}

// goalsRunner is a runner on g watching channel "a" with the given points
// mode and community goals setting.
func goalsRunner(t *testing.T, g *goalsFake, mode platform.PointsMode, on *atomic.Bool) *Runner {
	t.Helper()
	g.Campaigns = []platform.Campaign{camp("c1", "Rust", "d1")}
	g.Campaigns[0].Benefits[0].RequiredMinutes = 100000
	g.Progress["d1"] = &platform.Progress{BenefitID: "d1"}
	g.Streams["c1"] = []platform.Stream{{Channel: "a", ChannelID: "ida", ViewerCount: 10}}
	cfg := Config{AccountID: "acc", Platform: "twitch", Backend: g, Games: rustGames,
		PointsMode: fixedPoints(mode), TickInterval: time.Nanosecond}
	if on != nil {
		cfg.CommunityGoals = on.Load
	}
	r := New(cfg)
	t.Cleanup(func() { r.stopWatch(context.Background()) })
	return r
}

// With points on and the setting on, each points claim is followed by a
// contribution on the watched channel.
func TestGoals_ContributeWithPointsClaim(t *testing.T) {
	shrinkPoints(t, 20*time.Millisecond, time.Second)
	g := newGoalsFake()
	r := goalsRunner(t, g, platform.PointsDropsPoints, onAtomic(true))
	steps(t, r, 2) // pick, start
	if !waitFor(func() bool { return len(g.contributed()) >= 2 }, 2*time.Second) {
		t.Fatalf("contributions=%v, want one per claim", g.contributed())
	}
	for _, ch := range g.contributed() {
		if ch != "a" {
			t.Fatalf("contributed on %q, want a", ch)
		}
	}
}

// The setting off or unset sends no contribution, even with points on.
func TestGoals_OffByDefault(t *testing.T) {
	shrinkPoints(t, 5*time.Millisecond, time.Second)
	for _, on := range []*atomic.Bool{nil, onAtomic(false)} {
		g := newGoalsFake()
		r := goalsRunner(t, g, platform.PointsDropsPoints, on)
		steps(t, r, 2)
		if !waitFor(func() bool { return g.count() >= 2 }, time.Second) {
			t.Fatal("points claims did not run")
		}
		r.stopWatch(context.Background())
		if c := g.contributed(); len(c) != 0 {
			t.Fatalf("contributed %v with the setting off", c)
		}
	}
}

// With points off there is no points task, so no contribution either.
func TestGoals_NeedPointsMode(t *testing.T) {
	shrinkPoints(t, 5*time.Millisecond, time.Second)
	g := newGoalsFake()
	r := goalsRunner(t, g, platform.PointsOff, onAtomic(true))
	steps(t, r, 4)
	time.Sleep(30 * time.Millisecond)
	if c := g.contributed(); len(c) != 0 {
		t.Fatalf("contributed %v with points off", c)
	}
}

// Turning the setting on applies at the next claim, keeping the watch.
func TestGoals_SettingReadLive(t *testing.T) {
	shrinkPoints(t, 10*time.Millisecond, time.Second)
	g := newGoalsFake()
	on := onAtomic(false)
	r := goalsRunner(t, g, platform.PointsDropsPoints, on)
	steps(t, r, 2)
	time.Sleep(30 * time.Millisecond)
	if c := g.contributed(); len(c) != 0 {
		t.Fatalf("contributed %v while off", c)
	}
	on.Store(true)
	if !waitFor(func() bool { return len(g.contributed()) >= 1 }, time.Second) {
		t.Fatal("no contribution after turning the setting on")
	}
	if r.State() != StateWatching {
		t.Fatalf("state=%s", r.State())
	}
}
