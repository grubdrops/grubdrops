package summary

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grubdrops/grubdrops/internal/notify"
	"github.com/grubdrops/grubdrops/internal/store"
	"github.com/grubdrops/grubdrops/internal/store/gen"
)

func TestNextMidnight_DST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	got := NextMidnight(time.Date(2026, 3, 8, 1, 30, 0, 0, ny), ny)
	assert.Equal(t, time.Date(2026, 3, 9, 0, 0, 0, 0, ny), got)
	assert.Equal(t, "2026-03-09T00:00:00-04:00", got.Format(time.RFC3339))
}

type sent struct {
	event  string
	fields map[string]any
}

type fakeNotifier struct {
	mu   sync.Mutex
	sent []sent
}

func (f *fakeNotifier) Notify(_ context.Context, e notify.Event, fields map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sent{e, fields})
	return nil
}

func (f *fakeNotifier) all() []sent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sent(nil), f.sent...)
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time  { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) set(t time.Time) { c.mu.Lock(); c.t = t; c.mu.Unlock() }

// waitFor polls cond until it holds or 2s pass.
func waitFor(cond func() bool) {
	for end := time.Now().Add(2 * time.Second); time.Now().Before(end) && !cond(); {
		time.Sleep(5 * time.Millisecond)
	}
}

func runSummary(t *testing.T, enabled bool) []sent {
	t.Helper()
	old := maxWait
	maxWait = 5 * time.Millisecond
	t.Cleanup(func() { maxWait = old })

	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	q := gen.New(db)
	settings := store.NewSettings(q)
	minutes := store.NewWatchMinutes(q)
	claims := store.NewClaimLog(q)
	require.NoError(t, settings.SetNotifyDailySummary(ctx, enabled))

	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	require.NoError(t, minutes.Add(ctx, "a1", "2026-05-10", 42))
	require.NoError(t, minutes.Add(ctx, "a2", "2026-05-10", 8))
	require.NoError(t, minutes.Add(ctx, "a1", "2026-05-11", 99))
	rec := func(id string, at time.Time) {
		require.NoError(t, claims.Record(ctx, store.ClaimEntry{
			AccountID: "a1", BenefitID: id, ClaimedAt: at,
			Platform: "twitch", Game: "Rust", BenefitName: "drop " + id,
		}))
	}
	rec("b1", time.Date(2026, 5, 10, 9, 0, 0, 0, loc))
	rec("b2", time.Date(2026, 5, 10, 23, 59, 0, 0, loc))
	rec("b3", time.Date(2026, 5, 9, 23, 0, 0, 0, loc))
	rec("b4", time.Date(2026, 5, 11, 0, 30, 0, 0, loc))

	clk := &clock{t: time.Date(2026, 5, 10, 23, 59, 59, 0, loc)}
	n := &fakeNotifier{}
	rctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		Run(rctx, Config{Settings: settings, Minutes: minutes, Claims: claims, Notifier: n,
			Accounts: func(context.Context) ([]gen.Account, error) {
				return []gen.Account{{ID: "a1", Platform: "twitch", DisplayName: "one", CustomName: "First"}}, nil
			},
			Zone: func() *time.Location { return loc }, Now: clk.now})
	}()
	// Let the loop spin a few cycles before midnight, then cross it.
	time.Sleep(20 * time.Millisecond)
	assert.Empty(t, n.all(), "nothing before midnight")
	clk.set(time.Date(2026, 5, 11, 0, 0, 1, 0, loc))
	waitFor(func() bool { return len(n.all()) > 0 })
	if !enabled {
		// Off case: give a wrongly-sent event time to show up.
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	<-done
	return n.all()
}

func TestSummary_SendsYesterday(t *testing.T) {
	got := runSummary(t, true)
	require.Len(t, got, 1)
	assert.Equal(t, notify.EventSummary, got[0].event)
	assert.Equal(t, "2026-05-10", got[0].fields["day"])
	assert.Equal(t, 50, got[0].fields["minutes"])
	assert.Equal(t, 2, got[0].fields["claims"])
	assert.Equal(t, "2026-05-10: 50 min watched, 2 drops claimed.", got[0].fields["msg"])

	r, ok := got[0].fields[notify.FieldReport].(*notify.SummaryReport)
	require.True(t, ok)
	assert.Equal(t, "2026-05-10", r.Day.Format("2006-01-02"))
	assert.Equal(t, 50, r.Minutes)
	assert.Equal(t, []notify.SummaryAccount{
		{Label: "First", Platform: "twitch", Minutes: 42},
		{Label: "a2", Minutes: 8},
	}, r.Accounts)
	assert.Equal(t, []notify.SummaryDrop{
		{Game: "Rust", Drop: "drop b1", Platform: "twitch", Account: "First"},
		{Game: "Rust", Drop: "drop b2", Platform: "twitch", Account: "First"},
	}, r.Drops)
}

func TestSummary_ToggleOffSendsNothing(t *testing.T) {
	assert.Empty(t, runSummary(t, false))
}

// A day with more than 500 claims lists every drop, and the count in the
// report matches the one-line count.
func TestSummary_ReportCoversBusyDay(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	q := gen.New(db)
	claims := store.NewClaimLog(q)
	loc := time.UTC
	start := time.Date(2026, 5, 10, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	const n = 650
	for i := 0; i < n; i++ {
		require.NoError(t, claims.Record(ctx, store.ClaimEntry{
			AccountID: "a1", BenefitID: fmt.Sprintf("b%04d", i), Platform: "twitch", Game: "Rust",
			BenefitName: "drop", ClaimedAt: start.Add(time.Duration(i) * time.Minute / 2),
		}))
	}
	// Claims after the day must not leak in, and must not hide the day's.
	for i := 0; i < 50; i++ {
		require.NoError(t, claims.Record(ctx, store.ClaimEntry{
			AccountID: "a1", BenefitID: fmt.Sprintf("late%02d", i), Platform: "twitch", Game: "Rust",
			ClaimedAt: end.Add(time.Duration(i+1) * time.Minute),
		}))
	}
	c := Config{Minutes: store.NewWatchMinutes(q), Claims: claims}
	r := c.report(ctx, start, end, "2026-05-10", "en", 0)
	require.NotNil(t, r)
	assert.Len(t, r.Drops, n)

	since, err := claims.CountSince(ctx, start)
	require.NoError(t, err)
	after, err := claims.CountSince(ctx, end)
	require.NoError(t, err)
	assert.Equal(t, since-after, len(r.Drops))
}
