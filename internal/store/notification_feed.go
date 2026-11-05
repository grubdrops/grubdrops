package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/store/gen"
)

// Feed limits: rows older than FeedMaxAge, or past the newest FeedKeep, are
// pruned.
const (
	FeedMaxAge = 30 * 24 * time.Hour
	FeedKeep   = 500
)

// FeedItem is one row of the in-app notification feed. A zero ReadAt is
// unread.
type FeedItem struct {
	ID           int64
	CreatedAt    time.Time
	Event        string
	Kind         string
	AccountID    string
	AccountLabel string
	Platform     string
	Title        string
	Body         string
	Game         string
	Campaign     string
	ImageURL     string
	Link         string
	ReadAt       time.Time
}

// NotificationFeed stores the in-app notification feed.
type NotificationFeed struct {
	Q *gen.Queries
}

func NewNotificationFeed(q *gen.Queries) *NotificationFeed { return &NotificationFeed{Q: q} }

func feedFrom(r gen.NotificationFeed) FeedItem {
	return FeedItem{
		ID: r.ID, CreatedAt: time.Unix(r.CreatedAt, 0), Event: r.Event, Kind: r.Kind,
		AccountID: r.AccountID, AccountLabel: r.AccountLabel, Platform: r.Platform,
		Title: r.Title, Body: r.Body, Game: r.Game, Campaign: r.Campaign,
		ImageURL: r.ImageUrl, Link: r.Link, ReadAt: fromNullUnix(r.ReadAt),
	}
}

// Add stores it as unread and returns the stored row. A zero CreatedAt is
// stored as now.
func (f *NotificationFeed) Add(ctx context.Context, it FeedItem) (FeedItem, error) {
	at := it.CreatedAt
	if at.IsZero() {
		at = time.Now()
	}
	r, err := f.Q.InsertFeedNotification(ctx, gen.InsertFeedNotificationParams{
		CreatedAt: at.Unix(), Event: it.Event, Kind: it.Kind,
		AccountID: it.AccountID, AccountLabel: it.AccountLabel, Platform: it.Platform,
		Title: it.Title, Body: it.Body, Game: it.Game, Campaign: it.Campaign,
		ImageUrl: it.ImageURL, Link: it.Link,
	})
	if err != nil {
		return FeedItem{}, err
	}
	return feedFrom(r), nil
}

// List returns up to limit rows, newest first, and the unread count.
func (f *NotificationFeed) List(ctx context.Context, limit int) ([]FeedItem, int, error) {
	rows, err := f.Q.ListFeedNotifications(ctx, int64(limit))
	if err != nil {
		return nil, 0, err
	}
	unread, err := f.Q.CountUnreadFeedNotifications(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]FeedItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, feedFrom(r))
	}
	return out, int(unread), nil
}

// MarkAllRead marks every unread row read at now and returns how many.
func (f *NotificationFeed) MarkAllRead(ctx context.Context, now time.Time) (int64, error) {
	return f.Q.MarkFeedNotificationsRead(ctx, sql.NullInt64{Int64: now.Unix(), Valid: true})
}

// Prune deletes rows older than FeedMaxAge before now and all but the newest
// FeedKeep.
func (f *NotificationFeed) Prune(ctx context.Context, now time.Time) error {
	if _, err := f.Q.DeleteFeedNotificationsBefore(ctx, now.Add(-FeedMaxAge).Unix()); err != nil {
		return err
	}
	_, err := f.Q.DeleteFeedNotificationsPastNewest(ctx, FeedKeep)
	return err
}
