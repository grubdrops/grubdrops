-- name: UpsertNeedsLinkDrop :exec
-- Keeps first_seen. Empty text fields and a zero ends_at keep the stored value.
INSERT INTO needs_link_drops (account_id, benefit_id, platform, game, campaign_id, campaign_name, benefit_name, image_url, link_url, ends_at, listed, first_seen, seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
ON CONFLICT(account_id, benefit_id) DO UPDATE SET
    platform      = excluded.platform,
    game          = CASE WHEN excluded.game <> '' THEN excluded.game ELSE needs_link_drops.game END,
    campaign_id   = CASE WHEN excluded.campaign_id <> '' THEN excluded.campaign_id ELSE needs_link_drops.campaign_id END,
    campaign_name = CASE WHEN excluded.campaign_name <> '' THEN excluded.campaign_name ELSE needs_link_drops.campaign_name END,
    benefit_name  = CASE WHEN excluded.benefit_name <> '' THEN excluded.benefit_name ELSE needs_link_drops.benefit_name END,
    image_url     = CASE WHEN excluded.image_url <> '' THEN excluded.image_url ELSE needs_link_drops.image_url END,
    link_url      = CASE WHEN excluded.link_url <> '' THEN excluded.link_url ELSE needs_link_drops.link_url END,
    ends_at       = CASE WHEN excluded.ends_at <> 0 THEN excluded.ends_at ELSE needs_link_drops.ends_at END,
    listed        = 1,
    seen_at       = excluded.seen_at;

-- name: ListNeedsLinkDropIDs :many
SELECT benefit_id FROM needs_link_drops WHERE account_id = ?;

-- name: MarkNeedsLinkDropListed :exec
UPDATE needs_link_drops SET listed = 1, seen_at = ? WHERE account_id = ? AND benefit_id = ?;

-- name: MarkNeedsLinkDropUnlisted :exec
UPDATE needs_link_drops SET listed = 0 WHERE account_id = ? AND benefit_id = ?;

-- name: DeleteNeedsLinkDrop :exec
DELETE FROM needs_link_drops WHERE account_id = ? AND benefit_id = ?;

-- name: PruneNeedsLinkDrops :exec
-- Unlisted drops whose campaign ended before the cutoff.
DELETE FROM needs_link_drops WHERE listed = 0 AND ends_at > 0 AND ends_at < ?;

-- name: ListUnclaimedNeedsLinkDrops :many
-- Every row the claim log does not hold.
SELECT * FROM needs_link_drops
WHERE NOT EXISTS (
    SELECT 1 FROM claim_log
    WHERE claim_log.account_id = needs_link_drops.account_id
      AND claim_log.benefit_id = needs_link_drops.benefit_id
)
ORDER BY account_id ASC, ends_at DESC, campaign_id ASC, benefit_id ASC;
