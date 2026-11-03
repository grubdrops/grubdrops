package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/grubdrops/grubdrops/internal/store/gen"
)

// NeedsLinkRetention is how long after its campaign ended a drop the
// platform no longer lists stays in needs_link_drops.
const NeedsLinkRetention = 30 * 24 * time.Hour

// NeedsLinkDrop is one drop watched to completion whose claim the platform
// refused until the account links a game account. Listed reports whether
// the latest inventory read still listed it unclaimed.
type NeedsLinkDrop struct {
	AccountID, Platform, Game, CampaignID, CampaignName string
	BenefitID, BenefitName, ImageURL, LinkURL           string
	EndsAt                                              time.Time
	Listed                                              bool
	FirstSeen, SeenAt                                   time.Time
}

// Expired reports whether the claim window has closed: the campaign ended
// and the platform's inventory no longer lists the drop.
func (d NeedsLinkDrop) Expired(now time.Time) bool {
	return !d.Listed && !d.EndsAt.IsZero() && !now.Before(d.EndsAt)
}

// NeedsLinkSync is one scan's update to an account's needs-link drops.
type NeedsLinkSync struct {
	// Refused are the completed drops the scan holds refused until linked.
	Refused []NeedsLinkDrop
	// Claimed are benefit IDs the scan shows claimed; their rows go.
	Claimed []string
	// Inventory is set when the scan read the inventory: Listed then holds
	// the benefit IDs it lists unclaimed, and every other row is unlisted.
	Inventory bool
	Listed    map[string]bool
}

// NeedsLinkDrops keeps completed drops refused for a missing link past the
// end of their campaign, until claimed or expired.
type NeedsLinkDrops struct {
	DB *sql.DB
	Q  *gen.Queries
}

func NewNeedsLinkDrops(db *sql.DB, q *gen.Queries) *NeedsLinkDrops {
	return &NeedsLinkDrops{DB: db, Q: q}
}

// Sync applies one scan of accountID in a single transaction, then prunes
// rows expired for longer than NeedsLinkRetention.
func (s *NeedsLinkDrops) Sync(ctx context.Context, accountID string, u NeedsLinkSync, now time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := s.Q.WithTx(tx)
	for _, id := range u.Claimed {
		if err := q.DeleteNeedsLinkDrop(ctx, gen.DeleteNeedsLinkDropParams{AccountID: accountID, BenefitID: id}); err != nil {
			return err
		}
	}
	refused := make(map[string]bool, len(u.Refused))
	for _, d := range u.Refused {
		refused[d.BenefitID] = true
		if err := q.UpsertNeedsLinkDrop(ctx, gen.UpsertNeedsLinkDropParams{
			AccountID: accountID, BenefitID: d.BenefitID, Platform: d.Platform, Game: d.Game,
			CampaignID: d.CampaignID, CampaignName: d.CampaignName, BenefitName: d.BenefitName,
			ImageUrl: d.ImageURL, LinkUrl: httpURL(d.LinkURL), EndsAt: unixOrZero(d.EndsAt),
			FirstSeen: now.Unix(), SeenAt: now.Unix(),
		}); err != nil {
			return err
		}
	}
	if u.Inventory {
		ids, err := q.ListNeedsLinkDropIDs(ctx, accountID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			switch {
			case refused[id]:
			case u.Listed[id]:
				err = q.MarkNeedsLinkDropListed(ctx, gen.MarkNeedsLinkDropListedParams{SeenAt: now.Unix(), AccountID: accountID, BenefitID: id})
			default:
				err = q.MarkNeedsLinkDropUnlisted(ctx, gen.MarkNeedsLinkDropUnlistedParams{AccountID: accountID, BenefitID: id})
			}
			if err != nil {
				return err
			}
		}
	}
	if err := q.PruneNeedsLinkDrops(ctx, now.Add(-NeedsLinkRetention).Unix()); err != nil {
		return err
	}
	return tx.Commit()
}

// Clear drops the account's row for benefitID, e.g. after a claim.
func (s *NeedsLinkDrops) Clear(ctx context.Context, accountID, benefitID string) error {
	return s.Q.DeleteNeedsLinkDrop(ctx, gen.DeleteNeedsLinkDropParams{AccountID: accountID, BenefitID: benefitID})
}

// Unclaimed lists every row the claim log does not hold, by account, then
// latest campaign end first.
func (s *NeedsLinkDrops) Unclaimed(ctx context.Context) ([]NeedsLinkDrop, error) {
	rows, err := s.Q.ListUnclaimedNeedsLinkDrops(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]NeedsLinkDrop, 0, len(rows))
	for _, r := range rows {
		out = append(out, NeedsLinkDrop{
			AccountID: r.AccountID, Platform: r.Platform, Game: r.Game,
			CampaignID: r.CampaignID, CampaignName: r.CampaignName,
			BenefitID: r.BenefitID, BenefitName: r.BenefitName, ImageURL: r.ImageUrl,
			LinkURL: httpURL(r.LinkUrl), EndsAt: timeOrZero(r.EndsAt), Listed: r.Listed != 0,
			FirstSeen: time.Unix(r.FirstSeen, 0), SeenAt: time.Unix(r.SeenAt, 0),
		})
	}
	return out, nil
}
