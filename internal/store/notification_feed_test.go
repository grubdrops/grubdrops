package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration0030NotificationFeedUpDown(t *testing.T) {
	ctx := context.Background()
	db := migratedTo(t, 30)
	_, err := db.ExecContext(ctx, `SELECT id, created_at, event, kind, account_id, account_label, platform,
		title, body, game, campaign, image_url, link, read_at FROM notification_feed`)
	require.NoError(t, err)
	require.NoError(t, goose.DownToContext(ctx, db, "migrations", 29))
	_, err = db.ExecContext(ctx, `SELECT 1 FROM notification_feed`)
	require.Error(t, err, "down drops the table")
	_, err = db.ExecContext(ctx, `SELECT 1 FROM notifications`)
	require.NoError(t, err, "the old table stays")
}

func TestNotificationFeed_AddListRead(t *testing.T) {
	_, q := openV2(t)
	f := NewNotificationFeed(q)
	ctx := context.Background()
	t0 := time.Unix(1_800_000_000, 0)

	a, err := f.Add(ctx, FeedItem{CreatedAt: t0, Event: "claim", Kind: "claim", AccountID: "a1",
		AccountLabel: "Alice", Platform: "twitch", Title: "Cape", Game: "Minecraft", Campaign: "Spring",
		ImageURL: "https://img/x.png"})
	require.NoError(t, err)
	assert.Positive(t, a.ID)
	assert.Equal(t, t0, a.CreatedAt)
	assert.True(t, a.ReadAt.IsZero())
	b, err := f.Add(ctx, FeedItem{CreatedAt: t0.Add(time.Minute), Event: "error", Kind: "needs_link",
		Title: "Skin", Link: "https://example.com/link"})
	require.NoError(t, err)

	rows, unread, err := f.List(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, unread)
	require.Len(t, rows, 2)
	assert.Equal(t, b.ID, rows[0].ID, "newest first")
	assert.Equal(t, "https://example.com/link", rows[0].Link)
	assert.Equal(t, a, rows[1])

	n, err := f.MarkAllRead(ctx, t0.Add(time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
	rows, unread, err = f.List(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, unread)
	assert.Equal(t, t0.Add(time.Hour), rows[0].ReadAt)

	n, err = f.MarkAllRead(ctx, t0.Add(2*time.Hour))
	require.NoError(t, err)
	assert.Zero(t, n, "read rows keep their first read time")
}

func TestNotificationFeed_Prune(t *testing.T) {
	_, q := openV2(t)
	f := NewNotificationFeed(q)
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0)

	_, err := f.Add(ctx, FeedItem{CreatedAt: now.Add(-FeedMaxAge - time.Minute), Event: "claim", Kind: "claim", Title: "old"})
	require.NoError(t, err)
	_, err = f.Add(ctx, FeedItem{CreatedAt: now.Add(-FeedMaxAge + time.Minute), Event: "claim", Kind: "claim", Title: "recent"})
	require.NoError(t, err)
	require.NoError(t, f.Prune(ctx, now))
	rows, _, err := f.List(ctx, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1, "past 30 days is dropped")
	assert.Equal(t, "recent", rows[0].Title)

	for i := range FeedKeep + 5 {
		_, err := f.Add(ctx, FeedItem{CreatedAt: now.Add(-time.Duration(FeedKeep-i) * time.Second),
			Event: "claim", Kind: "claim", Title: fmt.Sprint(i)})
		require.NoError(t, err)
	}
	require.NoError(t, f.Prune(ctx, now))

	rows, unread, err := f.List(ctx, FeedKeep+10)
	require.NoError(t, err)
	assert.Len(t, rows, FeedKeep)
	assert.Equal(t, FeedKeep, unread)
	assert.Equal(t, fmt.Sprint(FeedKeep+4), rows[0].Title, "newest kept")
	assert.Equal(t, "5", rows[len(rows)-1].Title, "oldest past the cap dropped")
}
