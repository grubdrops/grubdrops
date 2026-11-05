-- +goose Up
-- Drops watched to completion whose claim the platform refused until the
-- user links a game account. Unlike drop_states, a row outlives its
-- campaign: it goes when the drop is claimed, or 30 days after the campaign
-- ended once the platform's inventory stops listing it.
-- listed is 1 while the latest inventory read still lists the drop
-- unclaimed. seen_at is the last time it did. Times are Unix seconds.
CREATE TABLE needs_link_drops (
    account_id    TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    benefit_id    TEXT NOT NULL,
    platform      TEXT NOT NULL,
    game          TEXT NOT NULL DEFAULT '',
    campaign_id   TEXT NOT NULL DEFAULT '',
    campaign_name TEXT NOT NULL DEFAULT '',
    benefit_name  TEXT NOT NULL DEFAULT '',
    image_url     TEXT NOT NULL DEFAULT '',
    link_url      TEXT NOT NULL DEFAULT '',
    ends_at       INTEGER NOT NULL DEFAULT 0,
    listed        INTEGER NOT NULL DEFAULT 1,
    first_seen    INTEGER NOT NULL,
    seen_at       INTEGER NOT NULL,
    PRIMARY KEY (account_id, benefit_id)
);

-- +goose Down
DROP TABLE IF EXISTS needs_link_drops;
