package api

import (
	"cmp"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/grubdrops/grubdrops/internal/store"
)

// EventNotifications: the bell feed changed. Data is notificationsEventJSON:
// a new row, or read set when every row was marked read.
const EventNotifications = "notifications"

// Feed list sizes for GET /api/notifications.
const (
	feedDefaultLimit = 100
	feedMaxLimit     = store.FeedKeep
)

// notificationJSON is one bell feed row.
type notificationJSON struct {
	ID          int64   `json:"id"`
	CreatedAt   *string `json:"created_at"`
	Event       string  `json:"event"`
	Kind        string  `json:"kind"`
	AccountID   string  `json:"account_id"`
	AccountName string  `json:"account_name"`
	Platform    string  `json:"platform"`
	Title       string  `json:"title"`
	Body        string  `json:"body"`
	Game        string  `json:"game"`
	Campaign    string  `json:"campaign"`
	ImageURL    string  `json:"image_url"`
	Link        string  `json:"link"`
	Read        bool    `json:"read"`
}

type notificationsEventJSON struct {
	Item *notificationJSON `json:"item,omitempty"`
	Read bool              `json:"read,omitempty"`
}

func notificationView(it store.FeedItem, name string) notificationJSON {
	return notificationJSON{
		ID: it.ID, CreatedAt: rfc3339(it.CreatedAt), Event: it.Event, Kind: it.Kind,
		AccountID: it.AccountID, AccountName: cmp.Or(name, it.AccountLabel, it.AccountID),
		Platform: it.Platform, Title: it.Title, Body: it.Body, Game: it.Game, Campaign: it.Campaign,
		ImageURL: rewardImage(it.Platform, it.ImageURL), Link: it.Link, Read: !it.ReadAt.IsZero(),
	}
}

// NotificationEvent is the SSE event for a newly stored feed row.
func NotificationEvent(it store.FeedItem) Event {
	v := notificationView(it, "")
	return Event{Name: EventNotifications, Data: notificationsEventJSON{Item: &v}}
}

func (s *server) routesNotifications(r chi.Router) {
	r.Get("/notifications", s.getNotifications)
	r.Post("/notifications/read", s.readNotifications)
}

// getNotifications lists the newest feed rows, newest first, with the unread
// count: ?limit=N, default 100.
func (s *server) getNotifications(w http.ResponseWriter, r *http.Request) {
	if s.d.Feed == nil {
		writeErr(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	limit := feedDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeErr(w, http.StatusBadRequest, "bad_limit")
			return
		}
		limit = min(n, feedMaxLimit)
	}
	ctx := r.Context()
	rows, unread, err := s.d.Feed.List(ctx, limit)
	if err != nil {
		slog.Warn("api: list notifications failed", "kind", "error", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	// Current account names, so a rename shows on old rows too.
	names := map[string]string{}
	if accts, err := s.d.Q.ListAllAccounts(ctx); err == nil {
		for _, a := range accts {
			names[a.ID] = store.AccountName(a)
		}
	}
	out := make([]notificationJSON, 0, len(rows))
	for _, it := range rows {
		out = append(out, notificationView(it, names[it.AccountID]))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "unread": unread})
}

// readNotifications marks every row read and tells other open tabs.
func (s *server) readNotifications(w http.ResponseWriter, r *http.Request) {
	if s.d.Feed == nil {
		writeErr(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	n, err := s.d.Feed.MarkAllRead(r.Context(), s.overviewNow())
	if err != nil {
		slog.Warn("api: mark notifications read failed", "kind", "error", "err", err)
		writeErr(w, http.StatusInternalServerError, "internal")
		return
	}
	if n > 0 && s.d.Hub != nil {
		s.d.Hub.Publish(Event{Name: EventNotifications, Data: notificationsEventJSON{Read: true}})
	}
	w.WriteHeader(http.StatusNoContent)
}
