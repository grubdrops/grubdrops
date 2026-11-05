-- name: InsertFeedNotification :one
INSERT INTO notification_feed (created_at, event, kind, account_id, account_label, platform, title, body, game, campaign, image_url, link)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListFeedNotifications :many
-- Newest first.
SELECT * FROM notification_feed
ORDER BY id DESC
LIMIT ?;

-- name: CountUnreadFeedNotifications :one
SELECT COUNT(*) FROM notification_feed WHERE read_at IS NULL;

-- name: MarkFeedNotificationsRead :execrows
UPDATE notification_feed SET read_at = ? WHERE read_at IS NULL;

-- name: DeleteFeedNotificationsBefore :execrows
DELETE FROM notification_feed WHERE created_at < ?;

-- name: DeleteFeedNotificationsPastNewest :execrows
-- Keeps the newest rows by id, as many as given, and deletes the rest.
DELETE FROM notification_feed
WHERE id NOT IN (SELECT id FROM notification_feed ORDER BY id DESC LIMIT ?);
