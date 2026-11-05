package feed

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aalejandrofer/grubdrops/internal/notify"
	"github.com/aalejandrofer/grubdrops/internal/store"
	"github.com/aalejandrofer/grubdrops/internal/store/gen"
)

func TestFromEvent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		event  string
		fields map[string]any
		want   store.FeedItem
		ok     bool
	}{
		{"claim", notify.EventClaim, map[string]any{
			"account": "a1", "account_label": "Alice", "platform": "twitch", "game": "Rust",
			"campaign": "Spring", "drop": "Cape", "image": "https://img/x.png", "channel": "ch", "req_min": 60,
		}, store.FeedItem{Event: "claim", Kind: KindClaim, AccountID: "a1", AccountLabel: "Alice",
			Platform: "twitch", Game: "Rust", Campaign: "Spring", Title: "Cape", ImageURL: "https://img/x.png"}, true},
		{"needs link", notify.EventError, map[string]any{
			"account": "b2", "platform": "kick", "drop": "Skin", "msg": "Link your game account",
			notify.FieldFeedKind: notify.FeedKindNeedsLink, notify.FieldFeedLink: "https://kick.com/connect",
		}, store.FeedItem{Event: "error", Kind: KindNeedsLink, AccountID: "b2", Platform: "kick",
			Title: "Skin", Link: "https://kick.com/connect"}, true},
		{"needs link, bad link dropped", notify.EventError, map[string]any{
			"drop": "Skin", notify.FieldFeedKind: notify.FeedKindNeedsLink, notify.FieldFeedLink: "javascript:alert(1)",
		}, store.FeedItem{Event: "error", Kind: KindNeedsLink, Title: "Skin"}, true},
		{"runner error", notify.EventError, map[string]any{"account": "a1", "error": "boom"},
			store.FeedItem{Event: "error", Kind: KindError, AccountID: "a1", Body: "boom"}, true},
		{"auth fail", notify.EventAuth, map[string]any{"account": "a1", "ok": false, "reason": "token expired",
			"platform": "twitch", "name": "Alice"},
			store.FeedItem{Event: "auth", Kind: KindAuth, AccountID: "a1", AccountLabel: "Alice", Platform: "twitch",
				Body: "token expired"}, true},
		{"auth back", notify.EventAuth, map[string]any{"account": "a1", "ok": true, "reason": ""},
			store.FeedItem{Event: "auth", Kind: KindAuthOK, AccountID: "a1"}, true},
		{"canary", notify.EventCanary, map[string]any{"platform": "kick", "detail": "no credit"},
			store.FeedItem{Event: "canary", Kind: KindCanary, Platform: "kick", Body: "no credit"}, true},
		{"reward code keeps no code", notify.EventRewardCode, map[string]any{
			"title": "New reward code", "reward": "Cape", "reward_game": "Minecraft", "account_label": "Alice",
			"code": "SECRET-CODE", "redeem_url": "https://minecraft.net/redeem", "platform": "twitch", "expires": "in 3 days",
		}, store.FeedItem{Event: "reward_code", Kind: KindRewardCode, AccountLabel: "Alice", Platform: "twitch",
			Title: "Cape", Game: "Minecraft", Body: "in 3 days"}, true},
		{"reward reminder", notify.EventRewardCode, map[string]any{"reward": "Cape",
			notify.FieldFeedKind: notify.FeedKindRewardReminder},
			store.FeedItem{Event: "reward_code", Kind: KindRewardReminder, Title: "Cape"}, true},
		{"kick daily reward", notify.EventClaim, map[string]any{"account": "k1", "platform": "kick",
			"drop": "Kick daily reward: Gold (rare)", notify.FieldFeedKind: notify.FeedKindDailyReward,
			notify.FieldFeedTitle: "Gold (rare)"},
			store.FeedItem{Event: "claim", Kind: KindDailyReward, AccountID: "k1", Platform: "kick", Title: "Gold (rare)"}, true},
		{"kick daily reward, unnamed", notify.EventClaim, map[string]any{"drop": "Kick daily reward claimed",
			notify.FieldFeedKind: notify.FeedKindDailyReward, notify.FieldFeedTitle: ""},
			store.FeedItem{Event: "claim", Kind: KindDailyReward}, true},
		{"progress skipped", notify.EventProgress, map[string]any{"drop": "Cape"}, store.FeedItem{}, false},
		{"state skipped", notify.EventState, map[string]any{}, store.FeedItem{}, false},
		{"test skipped", notify.EventTest, map[string]any{"drop": "Cape"}, store.FeedItem{}, false},
		{"summary skipped", notify.EventSummary, map[string]any{"msg": "x"}, store.FeedItem{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := FromEvent(tc.event, tc.fields)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func openFeed(t *testing.T) *store.NotificationFeed {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "f.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return store.NewNotificationFeed(gen.New(db))
}

// With no webhook or bot set up, the feed still records every kept event.
func TestTee_RecordsWithNoTarget(t *testing.T) {
	st := openFeed(t)
	now := time.Unix(1_800_000_000, 0)
	var added []store.FeedItem
	tee := &Tee{
		Inner: &notify.NoopNotifier{},
		Feed:  &Feed{Store: st, Now: func() time.Time { return now }, OnAdd: func(it store.FeedItem) { added = append(added, it) }},
	}
	ctx := context.Background()
	assert.False(t, notify.Enabled(ctx, tee, notify.EventRewardCode), "the feed never turns a send on")
	assert.False(t, notify.Enabled(ctx, tee, notify.EventProgress), "progress has no target")

	require.NoError(t, tee.Notify(ctx, notify.EventClaim, map[string]any{"account": "a1", "drop": "Cape"}))
	require.NoError(t, tee.Notify(ctx, notify.EventProgress, map[string]any{"drop": "Cape"}))
	require.NoError(t, tee.Notify(ctx, notify.EventRewardCode, map[string]any{"reward": "Cape", "code": "SECRET"}))

	rows, unread, err := st.List(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, unread)
	require.Len(t, rows, 2)
	assert.Equal(t, KindRewardCode, rows[0].Kind)
	assert.Equal(t, KindClaim, rows[1].Kind)
	assert.Equal(t, now, rows[1].CreatedAt)
	require.Len(t, added, 2)
	assert.Positive(t, added[0].ID, "OnAdd gets the stored row")
	for _, r := range rows {
		assert.NotContains(t, r.Body+r.Title, "SECRET")
	}
}

// A send aimed at one target, such as a settings test, skips the feed.
func TestTee_TargetedSendSkipsFeed(t *testing.T) {
	st := openFeed(t)
	tee := &Tee{Inner: &notify.Multi{}, Feed: &Feed{Store: st}}
	ctx := notify.WithTarget(context.Background(), notify.TargetDiscord)
	assert.ErrorIs(t, tee.Notify(ctx, notify.EventClaim, map[string]any{"drop": "Cape"}), notify.ErrNoTarget)
	assert.False(t, notify.Enabled(ctx, tee, notify.EventClaim))
	rows, _, err := st.List(context.Background(), 10)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// A send marked with notify.SkipFeed reaches Inner but not the feed.
func TestTee_SkipFeed(t *testing.T) {
	st := openFeed(t)
	inner := &spy{}
	tee := &Tee{Inner: inner, Feed: &Feed{Store: st}}
	ctx := notify.SkipFeed(context.Background())
	require.NoError(t, tee.Notify(ctx, notify.EventClaim, map[string]any{"drop": "Cape"}))
	require.Len(t, inner.fields, 1)
	rows, _, err := st.List(context.Background(), 10)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

type spy struct {
	mu     sync.Mutex
	fields []map[string]any
}

func (s *spy) Notify(_ context.Context, _ notify.Event, f map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fields = append(s.fields, f)
	return nil
}

// Inner gets the event as sent, minus the feed-only fields.
func TestTee_InnerGetsFieldsUnchanged(t *testing.T) {
	sp := &spy{}
	tee := &Tee{Inner: sp, Feed: &Feed{Store: openFeed(t)}}
	plain := map[string]any{"drop": "Cape", "msg": "m"}
	require.NoError(t, tee.Notify(context.Background(), notify.EventError, plain))
	withFeed := map[string]any{"drop": "Cape", "msg": "m",
		notify.FieldFeedKind: notify.FeedKindNeedsLink, notify.FieldFeedLink: "https://x.example/link"}
	require.NoError(t, tee.Notify(context.Background(), notify.EventError, withFeed))
	require.Len(t, sp.fields, 2)
	assert.Equal(t, plain, sp.fields[0])
	assert.Equal(t, plain, sp.fields[1])
	assert.Contains(t, withFeed, notify.FieldFeedKind, "the caller's map is not changed")
}

// The Discord payload through the tee matches a direct send of the same
// event without the feed-only fields, so the feed changes nothing there.
func TestTee_DiscordPayloadUnchanged(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		for _, e := range m["embeds"].([]any) {
			delete(e.(map[string]any), "timestamp")
		}
		mu.Lock()
		bodies = append(bodies, m)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	allow := &notify.VerbosityFilter{Allow: map[string]bool{notify.EventError: true, notify.EventClaim: true}}
	wh := notify.NewDiscordWebhook(srv.URL, allow)
	tee := &Tee{Inner: wh, Feed: &Feed{Store: openFeed(t)}}
	ctx := context.Background()

	base := map[string]any{"account": "a1", "account_label": "Alice", "platform": "kick", "game": "Rust",
		"campaign": "Spring", "drop": "Cape", "image": "https://img/x.png", "msg": "Link your game account"}
	require.NoError(t, wh.Notify(ctx, notify.EventError, base))
	withFeed := map[string]any{notify.FieldFeedKind: notify.FeedKindNeedsLink, notify.FieldFeedLink: "https://kick.com/c"}
	for k, v := range base {
		withFeed[k] = v
	}
	require.NoError(t, tee.Notify(ctx, notify.EventError, withFeed))
	// A filtered-off event stays off through the tee.
	off := notify.NewDiscordWebhook(srv.URL, &notify.VerbosityFilter{Allow: map[string]bool{}})
	require.NoError(t, (&Tee{Inner: off, Feed: &Feed{Store: openFeed(t)}}).Notify(ctx, notify.EventClaim, base))

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, bodies, 2, "the filtered-off claim never posts")
	assert.Equal(t, bodies[0], bodies[1])
}
