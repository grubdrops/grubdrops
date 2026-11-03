package store

import (
	"context"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration0030NeedsLinkDropsUpDown(t *testing.T) {
	ctx := context.Background()
	db := migratedTo(t, 30)
	_, err := db.ExecContext(ctx, `SELECT account_id, benefit_id, platform, game, campaign_id, campaign_name,
		benefit_name, image_url, link_url, ends_at, listed, first_seen, seen_at FROM needs_link_drops`)
	require.NoError(t, err)
	require.NoError(t, goose.DownToContext(ctx, db, "migrations", 29))
	_, err = db.ExecContext(ctx, `SELECT 1 FROM needs_link_drops`)
	require.Error(t, err, "down drops the table")
}

func needsLinkFixture(t *testing.T) (*NeedsLinkDrops, *ClaimLog) {
	t.Helper()
	db, q := openV2(t)
	seedPlatformAccount(t, q, "k1", "kick")
	seedPlatformAccount(t, q, "t1", "twitch")
	return NewNeedsLinkDrops(db, q), NewClaimLog(q)
}

func iemDrop(id string, ends time.Time) NeedsLinkDrop {
	return NeedsLinkDrop{
		Platform: "kick", Game: "Counter-Strike 2", CampaignID: "c-iem", CampaignName: "IEM Beijing 2026 - Day 2",
		BenefitID: id, BenefitName: "Sticker " + id, ImageURL: "https://files.kick.com/" + id + ".png",
		LinkURL: "https://kick.com/settings/connections", EndsAt: ends,
	}
}

func TestNeedsLinkDrops_SyncKeepsRowsPastCampaignEnd(t *testing.T) {
	ctx := context.Background()
	s, _ := needsLinkFixture(t)
	t0 := time.Unix(1_800_000_000, 0)
	ends := t0.Add(2 * time.Hour)

	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Refused: []NeedsLinkDrop{iemDrop("r1", ends), iemDrop("r2", ends)}}, t0))
	got, err := s.Unclaimed(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "k1", got[0].AccountID)
	assert.Equal(t, "IEM Beijing 2026 - Day 2", got[0].CampaignName)
	assert.Equal(t, "https://kick.com/settings/connections", got[0].LinkURL)
	assert.True(t, got[0].Listed)
	assert.False(t, got[0].Expired(t0))

	// The campaign ended: scans no longer hold it, but the inventory still
	// lists r1 unclaimed. r1 stays open, r2 expires.
	later := ends.Add(time.Hour)
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Inventory: true, Listed: map[string]bool{"r1": true}}, later))
	got, err = s.Unclaimed(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2, "rows outlive the campaign")
	byID := map[string]NeedsLinkDrop{}
	for _, d := range got {
		byID[d.BenefitID] = d
	}
	assert.False(t, byID["r1"].Expired(later))
	assert.Equal(t, later.Unix(), byID["r1"].SeenAt.Unix())
	assert.True(t, byID["r2"].Expired(later))
	assert.Equal(t, t0.Unix(), byID["r2"].FirstSeen.Unix(), "first_seen is kept")

	// A scan without an inventory read leaves the listed flags alone.
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{}, later.Add(time.Minute)))
	got, err = s.Unclaimed(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// 30 days after the end the expired row is pruned, the listed one stays.
	require.NoError(t, s.Sync(ctx, "t1", NeedsLinkSync{}, ends.Add(NeedsLinkRetention+time.Minute)))
	got, err = s.Unclaimed(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "r1", got[0].BenefitID)
}

func TestNeedsLinkDrops_ClaimClears(t *testing.T) {
	ctx := context.Background()
	s, log := needsLinkFixture(t)
	t0 := time.Unix(1_800_000_000, 0)
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Refused: []NeedsLinkDrop{iemDrop("r1", t0), iemDrop("r2", t0), iemDrop("r3", t0)}}, t0))

	// A claim-log row hides the drop without touching the table.
	require.NoError(t, log.Record(ctx, ClaimEntry{AccountID: "k1", BenefitID: "r1", Platform: "kick", Source: ClaimSourceGrubDrops, ClaimedAt: t0}))
	// A scan that shows r2 claimed deletes it; Clear deletes r3.
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Claimed: []string{"r2"}}, t0))
	require.NoError(t, s.Clear(ctx, "k1", "r3"))

	got, err := s.Unclaimed(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestNeedsLinkDrops_UpsertKeepsKnownFields(t *testing.T) {
	ctx := context.Background()
	s, _ := needsLinkFixture(t)
	t0 := time.Unix(1_800_000_000, 0)
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Refused: []NeedsLinkDrop{iemDrop("r1", t0)}}, t0))
	bare := NeedsLinkDrop{Platform: "kick", BenefitID: "r1", LinkURL: "javascript:alert(1)"}
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Refused: []NeedsLinkDrop{bare}}, t0.Add(time.Minute)))
	got, err := s.Unclaimed(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Sticker r1", got[0].BenefitName)
	assert.Equal(t, "https://kick.com/settings/connections", got[0].LinkURL, "an unsafe URL never replaces the stored one")
	assert.Equal(t, t0.Unix(), got[0].EndsAt.Unix())
}

func TestNeedsLinkDrops_AccountDeleteCascades(t *testing.T) {
	ctx := context.Background()
	s, _ := needsLinkFixture(t)
	t0 := time.Unix(1_800_000_000, 0)
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Refused: []NeedsLinkDrop{iemDrop("r1", t0)}}, t0))
	_, err := s.DB.ExecContext(ctx, `DELETE FROM accounts WHERE id = 'k1'`)
	require.NoError(t, err)
	got, err := s.Unclaimed(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}
