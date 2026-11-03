package runner

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grubdrops/grubdrops/internal/i18n"
	"github.com/grubdrops/grubdrops/internal/platform"
	"github.com/grubdrops/grubdrops/internal/platform/platformtest"
)

// dailyKick is a backend with the daily reward claim. ready lists the
// claimable reward IDs; fail lists those whose claim fails.
type dailyKick struct {
	platform.Backend
	mu     sync.Mutex
	ready  []string
	fail   map[string]bool
	reward string
	calls  int
	posts  []string
}

func (k *dailyKick) ClaimDailyRewards(_ context.Context, _ platform.Session, skip func(string) bool) ([]platform.DailyReward, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.calls++
	var (
		got  []platform.DailyReward
		errs []error
	)
	for _, id := range k.ready {
		if skip != nil && skip(id) {
			continue
		}
		k.posts = append(k.posts, id)
		if k.fail[id] {
			errs = append(errs, &platform.DailyRewardError{ID: id, Err: errors.New("status 422")})
			continue
		}
		got = append(got, platform.DailyReward{ID: id, Name: "Daily Reward", Reward: k.reward})
	}
	return got, errors.Join(errs...)
}

func (k *dailyKick) callCount() int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.calls
}

func dailyRunner(k *dailyKick, on func() bool, n Notifier) *Runner {
	return New(Config{AccountID: "acc", Platform: "kick", Backend: k, Games: rustGames,
		KickDailyReward: on, Notifier: n})
}

// Off, or no setting at all, sends nothing; on, it claims.
func TestDailyReward_Gate(t *testing.T) {
	k := &dailyKick{Backend: platformtest.NewFake(), ready: []string{"d1"}}
	dailyRunner(k, nil, nil).claimDailyRewards(context.Background(), k)
	dailyRunner(k, func() bool { return false }, nil).claimDailyRewards(context.Background(), k)
	if k.calls != 0 {
		t.Fatalf("off: calls=%d", k.calls)
	}
	dailyRunner(k, func() bool { return true }, nil).claimDailyRewards(context.Background(), k)
	if k.calls != 1 || len(k.posts) != 1 {
		t.Fatalf("on: calls=%d posts=%v", k.calls, k.posts)
	}
}

// A reward whose claim failed is not claimed again this run; the others are.
func TestDailyReward_FailedNotRetried(t *testing.T) {
	k := &dailyKick{Backend: platformtest.NewFake(), ready: []string{"bad", "ok"}, fail: map[string]bool{"bad": true}}
	r := dailyRunner(k, func() bool { return true }, nil)
	r.claimDailyRewards(context.Background(), k)
	r.claimDailyRewards(context.Background(), k)
	want := []string{"bad", "ok", "ok"}
	if len(k.posts) != len(want) {
		t.Fatalf("posts=%v want %v", k.posts, want)
	}
	for i := range want {
		if k.posts[i] != want[i] {
			t.Fatalf("posts=%v want %v", k.posts, want)
		}
	}
}

// A claim notifies as a claim event, titled with the reward when Kick names
// it, in the UI language.
func TestDailyReward_Notifies(t *testing.T) {
	n := &spyNotifier{}
	k := &dailyKick{Backend: platformtest.NewFake(), ready: []string{"d1"}, reward: "Gold"}
	r := New(Config{AccountID: "acc", Platform: "kick", Backend: k, Games: rustGames, Notifier: n,
		KickDailyReward: func() bool { return true }, Lang: func() string { return "es" }})
	r.claimDailyRewards(context.Background(), k)
	got := n.of("claim")
	if len(got) != 1 {
		t.Fatalf("claims=%v", n.got)
	}
	want := "Gold"
	if title, _ := got[0].fields["drop"].(string); title == "" || title == i18n.T("es", "notify.kick_daily_reward_claimed") || !strings.Contains(title, want) {
		t.Fatalf("title=%q", got[0].fields["drop"])
	}
	if got[0].fields["platform"] != "kick" || got[0].fields["account"] != "acc" {
		t.Fatalf("fields=%v", got[0].fields)
	}

	n2 := &spyNotifier{}
	k2 := &dailyKick{Backend: platformtest.NewFake(), ready: []string{"d1"}}
	dailyRunner(k2, func() bool { return true }, n2).claimDailyRewards(context.Background(), k2)
	if g := n2.of("claim"); len(g) != 1 || g[0].fields["drop"] != i18n.T("en", "notify.kick_daily_reward_claimed") {
		t.Fatalf("no reward name: %v", n2.got)
	}
}

// Run checks at start while the setting is on, and sends nothing while it is
// off. Each check reads the setting (TestDailyReward_Gate).
func TestDailyReward_RunChecksAtStart(t *testing.T) {
	var on atomic.Bool
	k := &dailyKick{Backend: platformtest.NewFake(), ready: []string{"d1"}}
	r := dailyRunner(k, on.Load, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = r.Run(ctx)
	}()
	time.Sleep(50 * time.Millisecond)
	if k.callCount() != 0 {
		cancel()
		<-done
		t.Fatalf("off at start: calls=%d", k.callCount())
	}
	cancel()
	<-done

	on.Store(true)
	r = dailyRunner(k, on.Load, nil)
	ctx, cancel = context.WithCancel(context.Background())
	done = make(chan struct{})
	go func() {
		defer close(done)
		_ = r.Run(ctx)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for k.callCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if k.callCount() != 1 {
		t.Fatalf("on at start: calls=%d", k.callCount())
	}
}
