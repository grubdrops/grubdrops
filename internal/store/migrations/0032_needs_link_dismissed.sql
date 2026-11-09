-- +goose Up
-- dismissed_at is when the user hid the drop's group from Inventory, 0 while
-- shown. A refusal recorded again keeps it. A claim still deletes the row
-- and the 30 day prune still drops it. Unix seconds.
ALTER TABLE needs_link_drops ADD COLUMN dismissed_at INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE needs_link_drops DROP COLUMN dismissed_at;
