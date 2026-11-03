// Package feed records notifier events into the in-app notification feed
// behind the top bar bell. The feed is its own target: it records every event
// worth showing whatever Discord and Telegram are set to.
package feed

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/aalejandrofer/grubdrops/internal/notify"
	"github.com/aalejandrofer/grubdrops/internal/store"
)

// Row kinds. Each picks the row's icon, text and click target in the UI.
const (
	KindClaim          = "claim"
	KindNeedsLink      = notify.FeedKindNeedsLink
	KindError          = "error"
	KindAuth           = "auth"
	KindAuthOK         = "auth_ok"
	KindCanary         = "canary"
	KindRewardCode     = "reward_code"
	KindRewardReminder = notify.FeedKindRewardReminder
)

// Records reports whether the feed keeps event. Progress, state, test and
// summary events are noise in the feed.
func Records(event notify.Event) bool {
	switch event {
	case notify.EventClaim, notify.EventError, notify.EventAuth, notify.EventCanary, notify.EventRewardCode:
		return true
	}
	return false
}

// FromEvent maps a notifier event to a feed row. ok is false for an event
// the feed does not keep. A reward code's code is never kept.
func FromEvent(event notify.Event, fields map[string]any) (it store.FeedItem, ok bool) {
	if !Records(event) {
		return store.FeedItem{}, false
	}
	str := func(k string) string {
		s, _ := fields[k].(string)
		return strings.TrimSpace(s)
	}
	it = store.FeedItem{
		Event:        event,
		AccountID:    str("account"),
		AccountLabel: firstNonEmpty(str("account_label"), str("name")),
		Platform:     str("platform"),
		Game:         str("game"),
		Campaign:     str("campaign"),
		ImageURL:     str("image"),
	}
	switch event {
	case notify.EventClaim:
		it.Kind = KindClaim
		it.Title = str("drop")
	case notify.EventError:
		it.Title = str("drop")
		if str(notify.FieldFeedKind) == notify.FeedKindNeedsLink {
			it.Kind = KindNeedsLink
			it.Link = webURL(str(notify.FieldFeedLink))
		} else {
			it.Kind = KindError
			it.Body = firstNonEmpty(str("msg"), str("error"), str("panic"))
		}
	case notify.EventAuth:
		it.Kind = KindAuth
		if ok, _ := fields["ok"].(bool); ok {
			it.Kind = KindAuthOK
		} else {
			it.Body = str("reason")
		}
	case notify.EventCanary:
		it.Kind = KindCanary
		it.Body = str("detail")
	case notify.EventRewardCode:
		it.Kind = KindRewardCode
		if str(notify.FieldFeedKind) == KindRewardReminder {
			it.Kind = KindRewardReminder
		}
		it.Title = str("reward")
		it.Game = str("reward_game")
		it.Body = str("expires")
	}
	return it, true
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

// webURL returns raw when it is an absolute http(s) URL, else "".
func webURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return raw
}

// Feed stores feed rows and announces each new one.
type Feed struct {
	Store *store.NotificationFeed
	// OnAdd is called with each stored row; nil-safe. It must not block.
	OnAdd func(store.FeedItem)
	Now   func() time.Time // nil = time.Now
}

func (f *Feed) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

// Record stores event when the feed keeps it, prunes old rows and calls
// OnAdd. A store failure is logged, never returned: the feed must not hold up
// a notification.
func (f *Feed) Record(ctx context.Context, event notify.Event, fields map[string]any) {
	if f == nil || f.Store == nil {
		return
	}
	it, ok := FromEvent(event, fields)
	if !ok {
		return
	}
	now := f.now()
	it.CreatedAt = now
	// Detached: a cancelled run ctx must not drop the row.
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	saved, err := f.Store.Add(sctx, it)
	if err != nil {
		slog.Warn("feed: save notification failed", "kind", "error", "event", event, "err", err)
		return
	}
	if err := f.Store.Prune(sctx, now); err != nil {
		slog.Warn("feed: prune failed", "kind", "error", "err", err)
	}
	if f.OnAdd != nil {
		f.OnAdd(saved)
	}
}

// Recorder records one notifier event in the feed.
type Recorder interface {
	Record(ctx context.Context, event notify.Event, fields map[string]any)
}

// Tee sends every event to Inner unchanged, minus the feed-only fields, and
// records it in Feed. A send aimed at one target with notify.WithTarget, such
// as a settings test, skips the feed.
type Tee struct {
	Inner notify.Notifier
	Feed  Recorder
}

func (t *Tee) Notify(ctx context.Context, event notify.Event, fields map[string]any) error {
	if t.Feed != nil && notify.TargetFrom(ctx) == "" {
		t.Feed.Record(ctx, event, fields)
	}
	if t.Inner == nil {
		return nil
	}
	return t.Inner.Notify(ctx, event, notify.Strip(fields))
}

// Enabled is true for an event the feed keeps, since the feed always takes
// it. Otherwise, and for a send aimed at one target, Inner decides.
func (t *Tee) Enabled(ctx context.Context, event notify.Event) bool {
	if t.Feed != nil && notify.TargetFrom(ctx) == "" && Records(event) {
		return true
	}
	return notify.Enabled(ctx, t.Inner, event)
}
