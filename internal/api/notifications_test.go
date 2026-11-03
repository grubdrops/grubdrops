package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grubdrops/grubdrops/internal/store"
)

func newFeedEnv(t *testing.T) (*resEnv, *store.NotificationFeed) {
	t.Helper()
	var f *store.NotificationFeed
	e := newResEnv(t, func(d *Deps) {
		f = store.NewNotificationFeed(d.Q)
		d.Feed = f
		d.Now = func() time.Time { return rewardNow }
	})
	e.s.routesNotifications(e.s.protected)
	return e, f
}

type feedResp struct {
	Items  []map[string]any `json:"items"`
	Unread int              `json:"unread"`
}

func (e *resEnv) feed(path string) feedResp {
	e.t.Helper()
	resp, b := e.do("GET", path, "", nil)
	require.Equal(e.t, 200, resp.StatusCode, string(b))
	assert.Equal(e.t, "no-store", resp.Header.Get("Cache-Control"))
	var got feedResp
	require.NoError(e.t, json.Unmarshal(b, &got))
	return got
}

func TestNotifications_ListNewestFirst(t *testing.T) {
	e, f := newFeedEnv(t)
	ctx := context.Background()
	_, err := f.Add(ctx, store.FeedItem{CreatedAt: rewardNow.Add(-2 * time.Hour), Event: "claim", Kind: "claim",
		AccountID: "a1", AccountLabel: "old name", Platform: "twitch", Title: "Cape", Game: "Minecraft",
		ImageURL: "https://img.example/cape.png"})
	require.NoError(t, err)
	_, err = f.Add(ctx, store.FeedItem{CreatedAt: rewardNow.Add(-time.Hour), Event: "error", Kind: "needs_link",
		AccountID: "b2", Platform: "kick", Title: "Skin", Link: "https://example.com/link",
		ImageURL: "drops/reward-image/skin.png"})
	require.NoError(t, err)
	_, err = f.Add(ctx, store.FeedItem{CreatedAt: rewardNow, Event: "canary", Kind: "canary", Platform: "kick",
		Body: "no credit", AccountLabel: ""})
	require.NoError(t, err)

	got := e.feed("/api/notifications")
	require.Len(t, got.Items, 3)
	assert.Equal(t, 3, got.Unread)
	assert.Equal(t, "canary", got.Items[0]["kind"])
	assert.Equal(t, "", got.Items[0]["account_name"], "no account")
	nl := got.Items[1]
	assert.Equal(t, "needs_link", nl["kind"])
	assert.Equal(t, "Bob", nl["account_name"])
	assert.Equal(t, "https://example.com/link", nl["link"])
	assert.Equal(t, "/api/img/kick?u=drops%2Freward-image%2Fskin.png", nl["image_url"], "Kick art goes through the proxy")
	c := got.Items[2]
	assert.Equal(t, "Alice", c["account_name"], "current name wins over the stored label")
	assert.Equal(t, "Cape", c["title"])
	assert.Equal(t, "Minecraft", c["game"])
	assert.Equal(t, "https://img.example/cape.png", c["image_url"])
	assert.Equal(t, "2026-11-01T10:00:00Z", c["created_at"])
	assert.Equal(t, false, c["read"])

	got = e.feed("/api/notifications?limit=1")
	require.Len(t, got.Items, 1)
	assert.Equal(t, 3, got.Unread, "the count covers every row")

	resp, _ := e.do("GET", "/api/notifications?limit=x", "", nil)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestNotifications_ReadMarksAllAndPublishes(t *testing.T) {
	e, f := newFeedEnv(t)
	_, err := f.Add(context.Background(), store.FeedItem{Event: "claim", Kind: "claim", AccountID: "a1", Title: "Cape"})
	require.NoError(t, err)
	ch, cancel := e.s.d.Hub.Subscribe()
	defer cancel()

	_, tok := e.session()
	resp, b := e.do("POST", "/api/notifications/read", tok, nil)
	require.Equal(t, 204, resp.StatusCode, string(b))

	got := e.feed("/api/notifications")
	assert.Equal(t, 0, got.Unread)
	require.Len(t, got.Items, 1)
	assert.Equal(t, true, got.Items[0]["read"])

	select {
	case ev := <-ch:
		assert.Equal(t, EventNotifications, ev.Name)
		raw, err := json.Marshal(ev.Data)
		require.NoError(t, err)
		assert.JSONEq(t, `{"read":true}`, string(raw))
	case <-time.After(time.Second):
		t.Fatal("no notifications event")
	}

	// Nothing left to mark: no event.
	resp, _ = e.do("POST", "/api/notifications/read", tok, nil)
	require.Equal(t, 204, resp.StatusCode)
	select {
	case ev := <-ch:
		t.Fatalf("unexpected event %v", ev.Name)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestNotifications_ReadNeedsCSRF(t *testing.T) {
	e, f := newFeedEnv(t)
	_, err := f.Add(context.Background(), store.FeedItem{Event: "claim", Kind: "claim", Title: "Cape"})
	require.NoError(t, err)
	resp, b := e.do("POST", "/api/notifications/read", "", nil)
	assert.Equal(t, 403, resp.StatusCode)
	assert.JSONEq(t, `{"error":"csrf"}`, string(b))
	assert.Equal(t, 1, e.feed("/api/notifications").Unread, "nothing marked")
}

func TestNotifications_NeedsLogin(t *testing.T) {
	e := newEnv(t)
	e.s.routesNotifications(e.s.protected)
	resp, _ := e.do("GET", "/api/notifications", "", nil)
	assert.Equal(t, 401, resp.StatusCode)
	_, tok := e.session()
	resp, _ = e.do("POST", "/api/notifications/read", tok, nil)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestNotifications_Unavailable(t *testing.T) {
	e := newResEnv(t, nil)
	e.s.routesNotifications(e.s.protected)
	resp, _ := e.do("GET", "/api/notifications", "", nil)
	assert.Equal(t, 503, resp.StatusCode)
	_, tok := e.session()
	resp, _ = e.do("POST", "/api/notifications/read", tok, nil)
	assert.Equal(t, 503, resp.StatusCode)
}

func TestNotificationEvent_Shape(t *testing.T) {
	ev := NotificationEvent(store.FeedItem{ID: 7, CreatedAt: rewardNow, Event: "claim", Kind: "claim",
		AccountID: "a1", AccountLabel: "Alice", Title: "Cape"})
	assert.Equal(t, EventNotifications, ev.Name)
	assert.Empty(t, ev.Key, "never coalesced: every row shows")
	raw, err := json.Marshal(ev.Data)
	require.NoError(t, err)
	var got struct {
		Item map[string]any `json:"item"`
	}
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, float64(7), got.Item["id"])
	assert.Equal(t, "Alice", got.Item["account_name"])
	assert.Equal(t, false, got.Item["read"])
}
