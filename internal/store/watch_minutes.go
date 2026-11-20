package store

import (
	"context"

	"github.com/grubdrops/grubdrops/internal/store/gen"
)

// WatchMinutes counts minutes watched per account and day (YYYY-MM-DD).
type WatchMinutes struct{ Q *gen.Queries }

func NewWatchMinutes(q *gen.Queries) *WatchMinutes { return &WatchMinutes{Q: q} }

func (w *WatchMinutes) Add(ctx context.Context, accountID, day string, minutes int) error {
	return w.Q.AddWatchMinutes(ctx, gen.AddWatchMinutesParams{AccountID: accountID, Day: day, Minutes: int64(minutes)})
}

// Total sums every account and every day, the empty day included.
func (w *WatchMinutes) Total(ctx context.Context) (int, error) {
	n, err := w.Q.SumAllWatchMinutes(ctx)
	return int(n), err
}

// Day sums every account for one day.
func (w *WatchMinutes) Day(ctx context.Context, day string) (int, error) {
	n, err := w.Q.SumWatchMinutesDay(ctx, day)
	return int(n), err
}

// AccountMinutes is one account's watch minutes on a day.
type AccountMinutes struct {
	AccountID string
	Minutes   int
}

// DayByAccount lists each account with minutes on one day, most first.
func (w *WatchMinutes) DayByAccount(ctx context.Context, day string) ([]AccountMinutes, error) {
	rows, err := w.Q.ListWatchMinutesDay(ctx, day)
	if err != nil {
		return nil, err
	}
	out := make([]AccountMinutes, 0, len(rows))
	for _, r := range rows {
		out = append(out, AccountMinutes{AccountID: r.AccountID, Minutes: int(r.Minutes)})
	}
	return out, nil
}
