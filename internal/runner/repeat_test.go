package runner

import (
	"context"
	"testing"
	"time"

	"github.com/grubdrops/grubdrops/internal/plan"
	"github.com/grubdrops/grubdrops/internal/platform"
	"github.com/grubdrops/grubdrops/internal/platform/platformtest"
)

// repeatCamp is a Twitch-style daily group: days of 60 minutes chained by
// preconditions, none of them in the backend's inventory.
func repeatCamp(days int, reset time.Duration) platform.Campaign {
	c := platform.Campaign{ID: "chest", Platform: "twitch", Game: "Rust", Status: "active", Kind: platform.KindRepeat, AccountLinked: true}
	for d := 1; d <= days; d++ {
		b := platform.DropBenefit{
			ID: "g:day0" + string(rune('0'+d)), CampaignID: "chest", RequiredMinutes: 60,
			Repeat: &platform.Repeat{GroupID: "g", Day: d, Days: days, Reset: reset, CategoryIDs: []string{"cat"}},
		}
		if d > 1 {
			b.Preconditions = []string{"g:day0" + string(rune('0'+d-1))}
		}
		c.Benefits = append(c.Benefits, b)
	}
	return c
}

func repeatRunner(t *testing.T, claims ClaimLog, days int, reset time.Duration) (*Runner, *platformtest.Fake) {
	t.Helper()
	f := platformtest.NewFake()
	f.Campaigns = []platform.Campaign{repeatCamp(days, reset)}
	f.Streams["chest"] = []platform.Stream{{Channel: "a", ChannelID: "ida", GameID: "cat", ViewerCount: 5}}
	r := newRunner(f, func(c *Config) { c.Platform = "twitch"; c.Claims = claims })
	steps(t, r, 2)
	if r.State() != StateWatching {
		t.Fatalf("state=%s", r.State())
	}
	return r, f
}

func TestRepeat_LocalCountClaimsAfterRequiredPlusMargin(t *testing.T) {
	claims := &fakeClaims{ids: map[string]bool{}}
	r, f := repeatRunner(t, claims, 3, 24*time.Hour)
	// Past neverEnrolledThreshold: no backend row must not skip the day.
	steps(t, r, 60+repeatMargin-1)
	if r.State() != StateWatching {
		t.Fatalf("state=%s after %d ticks", r.State(), 60+repeatMargin-1)
	}
	steps(t, r, 1)
	if r.State() != StateClaiming {
		t.Fatalf("state=%s, want claiming", r.State())
	}
	steps(t, r, 1)
	if !hasCall(f, "Claim:g:day01") {
		t.Fatalf("calls=%v", f.Calls)
	}
	claims.mu.Lock()
	defer claims.mu.Unlock()
	if len(claims.recs) != 1 || claims.recs[0].Target.Benefit.ID != "g:day01" {
		t.Fatalf("recs=%+v", claims.recs)
	}
}

func TestRepeat_WrongCategoryDoesNotCount(t *testing.T) {
	r := &Runner{}
	tgt := plan.Target{Benefit: repeatCamp(2, time.Hour).Benefits[0]}
	for i := 0; i < 10; i++ {
		rows := r.repeatTick(tgt, &platform.Stream{GameID: "other"}, nil)
		if rows[0].MinutesWatched != 0 {
			t.Fatalf("minutes=%d", rows[0].MinutesWatched)
		}
	}
	rows := r.repeatTick(tgt, &platform.Stream{GameID: "cat"}, nil)
	if len(rows) != 1 {
		t.Fatalf("rows=%v", rows)
	}
}

func TestRepeat_EmptyStreamGameDoesNotCount(t *testing.T) {
	r := &Runner{}
	tgt := plan.Target{Benefit: repeatCamp(2, time.Hour).Benefits[0]}
	r.repeatTick(tgt, &platform.Stream{}, nil)
	r.repeatTick(tgt, nil, nil)
	if got := r.repeatMin[tgt.Benefit.ID]; got != 0 {
		t.Fatalf("count=%d, want 0", got)
	}
	free := tgt
	rep := *tgt.Benefit.Repeat
	rep.CategoryIDs = nil
	free.Benefit.Repeat = &rep
	r.repeatTick(free, &platform.Stream{}, nil)
	if got := r.repeatMin[tgt.Benefit.ID]; got != 1 {
		t.Fatalf("count=%d, want 1 when the group names no category", got)
	}
}

func TestRepeat_BackendRowKeepsTwitchGate(t *testing.T) {
	// Live: Twitch reopens the day at its window expiry, earlier than the
	// logged claim plus 24h, so the log gate must not hold a backend row.
	loggedAt := time.Now().Add(-time.Hour)
	claims := &correctingClaims{
		fakeClaims: &fakeClaims{ids: map[string]bool{"g:day01": true}},
		at:         map[string]time.Time{"g:day01": loggedAt},
	}
	r := New(Config{AccountID: "acc", Platform: "twitch", Backend: platformtest.NewFake(), Games: rustGames, Claims: claims})
	camps := []platform.Campaign{repeatCamp(3, 24*time.Hour)}
	windowEnd := loggedAt.Add(20 * time.Hour)
	backend := []platform.Progress{{BenefitID: "g:day02", MinutesWatched: 5, NextEligibleAt: windowEnd}}
	rows := r.withRepeatLocal(context.Background(), camps, backend, map[string]bool{"g:day01": true})
	if len(rows) != 1 || !rows[0].NextEligibleAt.Equal(windowEnd) || rows[0].MinutesWatched != 5 {
		t.Fatalf("rows=%+v, want Twitch's window gate kept", rows)
	}
	rows = r.withRepeatLocal(context.Background(), camps, nil, map[string]bool{"g:day01": true})
	if len(rows) != 1 || rows[0].BenefitID != "g:day02" || !rows[0].NextEligibleAt.Equal(loggedAt.Add(24*time.Hour)) {
		t.Fatalf("rows=%+v, want the log gate on a day Twitch gave no row", rows)
	}
}

func TestRepeat_LogsDaysTwitchReportsClaimed(t *testing.T) {
	claims := &fakeClaims{ids: map[string]bool{}}
	r := New(Config{AccountID: "acc", Platform: "twitch", Backend: platformtest.NewFake(), Games: rustGames, Claims: claims})
	camps := []platform.Campaign{repeatCamp(3, 24*time.Hour)}
	prog := []platform.Progress{{BenefitID: "g:day01", Claimed: true}, {BenefitID: "g:day02", Claimed: true}}
	got := r.logRepeatClaimed(context.Background(), camps, prog, map[string]bool{"g:day01": true})
	claims.mu.Lock()
	defer claims.mu.Unlock()
	if len(claims.recs) != 1 || claims.recs[0].Target.Benefit.ID != "g:day02" {
		t.Fatalf("recs=%+v, want only the unlogged day 2", claims.recs)
	}
	if !got["g:day02"] || len(claims.ids) != 0 {
		t.Fatalf("claimed=%v ids=%v", got, claims.ids)
	}
}

func TestRepeat_BackendRowWins(t *testing.T) {
	r := &Runner{}
	tgt := plan.Target{Benefit: repeatCamp(2, time.Hour).Benefits[0]}
	in := []platform.Progress{{BenefitID: tgt.Benefit.ID, MinutesWatched: 7}}
	out := r.repeatTick(tgt, nil, in)
	if len(out) != 1 || out[0].MinutesWatched != 7 {
		t.Fatalf("out=%v", out)
	}
}

func TestRepeat_CounterResetsAfterResetInterval(t *testing.T) {
	r := &Runner{}
	tgt := plan.Target{Benefit: repeatCamp(2, time.Hour).Benefits[0]}
	cat := &platform.Stream{GameID: "cat"}
	for i := 0; i < 10; i++ {
		r.repeatTick(tgt, cat, nil)
	}
	if got := r.repeatMin[tgt.Benefit.ID]; got != 10 {
		t.Fatalf("count=%d", got)
	}
	r.repeatFirst[tgt.Benefit.ID] = time.Now().Add(-2 * time.Hour)
	r.repeatTick(tgt, cat, nil)
	if got := r.repeatMin[tgt.Benefit.ID]; got != 1 {
		t.Fatalf("count after reset=%d, want 1", got)
	}
}

func TestRepeat_WithRepeatLocalGatesOnLoggedClaim(t *testing.T) {
	loggedAt := time.Now().Add(-time.Hour)
	claims := &correctingClaims{
		fakeClaims: &fakeClaims{ids: map[string]bool{"g:day01": true}},
		at:         map[string]time.Time{"g:day01": loggedAt},
	}
	r := New(Config{AccountID: "acc", Platform: "twitch", Backend: platformtest.NewFake(), Games: rustGames, Claims: claims})
	camps := []platform.Campaign{repeatCamp(3, 24*time.Hour)}
	rows := r.withRepeatLocal(context.Background(), camps, nil, map[string]bool{"g:day01": true})
	if len(rows) != 1 || rows[0].BenefitID != "g:day02" {
		t.Fatalf("rows=%+v, want a row for day 2 only (day 1 claimed, day 3 has no logged predecessor)", rows)
	}
	if want := loggedAt.Add(24 * time.Hour); !rows[0].NextEligibleAt.Equal(want) {
		t.Fatalf("next=%v want %v", rows[0].NextEligibleAt, want)
	}
}

func TestRepeat_ScanCapsNextStartAtCooldown(t *testing.T) {
	loggedAt := time.Now().Add(-time.Hour)
	claims := &correctingClaims{
		fakeClaims: &fakeClaims{ids: map[string]bool{"g:day01": true}},
		at:         map[string]time.Time{"g:day01": loggedAt},
	}
	f := platformtest.NewFake()
	f.Campaigns = []platform.Campaign{repeatCamp(3, 24*time.Hour)}
	r := New(Config{AccountID: "acc", Platform: "twitch", Backend: f, Games: rustGames, Claims: claims})
	res, err := r.scan(context.Background(), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Queue) != 0 {
		t.Fatalf("queue=%v: day 2 must wait for its window", ids(res.Queue))
	}
	r.mu.Lock()
	next := r.nextStart
	r.mu.Unlock()
	if want := loggedAt.Add(24 * time.Hour); !next.Equal(want) {
		t.Fatalf("nextStart=%v want %v", next, want)
	}
}

func ids(q []plan.Target) []string {
	var out []string
	for _, t := range q {
		out = append(out, t.Benefit.ID)
	}
	return out
}
