-- +goose Up
-- The in-app notification feed behind the top bar bell. One row per
-- notifier event worth showing: claims, link notices, login trouble,
-- failed health checks and reward codes. kind is the row's look in the UI;
-- event is the notifier event it came from. title names the drop, reward
-- or platform; body carries the detail text. link is an outside URL to
-- open on click, empty when the UI picks its own page. read_at is NULL
-- until the bell is opened. Rows older than 30 days, or past the newest
-- 500, are pruned.
CREATE TABLE notification_feed (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at    INTEGER NOT NULL,
    event         TEXT NOT NULL,
    kind          TEXT NOT NULL,
    account_id    TEXT NOT NULL DEFAULT '',
    account_label TEXT NOT NULL DEFAULT '',
    platform      TEXT NOT NULL DEFAULT '',
    title         TEXT NOT NULL DEFAULT '',
    body          TEXT NOT NULL DEFAULT '',
    game          TEXT NOT NULL DEFAULT '',
    campaign      TEXT NOT NULL DEFAULT '',
    image_url     TEXT NOT NULL DEFAULT '',
    link          TEXT NOT NULL DEFAULT '',
    read_at       INTEGER
);
CREATE INDEX notification_feed_created_at ON notification_feed (created_at);
CREATE INDEX notification_feed_unread ON notification_feed (read_at) WHERE read_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS notification_feed;
