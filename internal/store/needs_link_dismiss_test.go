package store

import (
	"context"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration0032NeedsLinkDismissedUpDown(t *testing.T) {
	ctx := context.Background()
	db := migratedTo(t, 32)
	_, err := db.ExecContext(ctx, `SELECT dismissed_at FROM needs_link_drops`)
	require.NoError(t, err)
	require.NoError(t, goose.DownToContext(ctx, db, "migrations", 31))
	_, err = db.ExecContext(ctx, `SELECT dismissed_at FROM needs_link_drops`)
	require.Error(t, err, "down drops the column")
}

func unclaimedIDs(t *testing.T, s *NeedsLinkDrops) []string {
	t.Helper()
	got, err := s.Unclaimed(context.Background())
	require.NoError(t, err)
	ids := []string{}
	for _, d := range got {
		ids = append(ids, d.AccountID+"/"+d.BenefitID)
	}
	return ids
}

func TestNeedsLinkDrops_DismissHidesGroup(t *testing.T) {
	ctx := context.Background()
	s, _ := needsLinkFixture(t)
	t0 := time.Unix(1_800_000_000, 0)
	other := iemDrop("o1", t0)
	other.CampaignID = "c-other"
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Refused: []NeedsLinkDrop{iemDrop("r1", t0), iemDrop("r2", t0), other}}, t0))
	require.NoError(t, s.Sync(ctx, "t1", NeedsLinkSync{Refused: []NeedsLinkDrop{iemDrop("r1", t0)}}, t0))

	ok, err := s.Dismiss(ctx, "k1", "c-iem", t0)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.ElementsMatch(t, []string{"k1/o1", "t1/r1"}, unclaimedIDs(t, s), "only k1's c-iem group is hidden")

	ok, err = s.Dismiss(ctx, "k1", "c-none", t0)
	require.NoError(t, err)
	assert.False(t, ok, "no row in that campaign")

	ok, err = s.Restore(ctx, "k1", "c-iem")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.ElementsMatch(t, []string{"k1/r1", "k1/r2", "k1/o1", "t1/r1"}, unclaimedIDs(t, s))
}

func TestNeedsLinkDrops_DismissSurvivesRefusal(t *testing.T) {
	ctx := context.Background()
	s, _ := needsLinkFixture(t)
	t0 := time.Unix(1_800_000_000, 0)
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Refused: []NeedsLinkDrop{iemDrop("r1", t0)}}, t0))
	_, err := s.Dismiss(ctx, "k1", "c-iem", t0)
	require.NoError(t, err)

	// The platform refuses r1 again, and r2 of the same campaign for the first time.
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{
		Refused:   []NeedsLinkDrop{iemDrop("r1", t0), iemDrop("r2", t0)},
		Inventory: true, Listed: map[string]bool{"r1": true, "r2": true},
	}, t0.Add(time.Hour)))
	assert.Empty(t, unclaimedIDs(t, s), "a re-refused row and a new row of the group stay hidden")

	var n int
	require.NoError(t, s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM needs_link_drops WHERE dismissed_at = ?`, t0.Unix()).Scan(&n))
	assert.Equal(t, 2, n, "the new row takes the group's dismissed_at")
}

func TestNeedsLinkDrops_DismissedClaimAndPrune(t *testing.T) {
	ctx := context.Background()
	s, _ := needsLinkFixture(t)
	t0 := time.Unix(1_800_000_000, 0)
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Refused: []NeedsLinkDrop{iemDrop("r1", t0), iemDrop("r2", t0), iemDrop("r3", t0)}}, t0.Add(-time.Hour)))
	_, err := s.Dismiss(ctx, "k1", "c-iem", t0)
	require.NoError(t, err)
	count := func() int {
		var n int
		require.NoError(t, s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM needs_link_drops`).Scan(&n))
		return n
	}

	// A claim still deletes a dismissed row, from a scan or the claim path.
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Claimed: []string{"r1"}}, t0))
	require.NoError(t, s.Clear(ctx, "k1", "r3"))
	assert.Equal(t, 1, count())

	// r2 drops off the inventory; 30 days after the end the prune deletes it.
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{Inventory: true, Listed: map[string]bool{}}, t0.Add(time.Hour)))
	assert.Equal(t, 1, count())
	require.NoError(t, s.Sync(ctx, "k1", NeedsLinkSync{}, t0.Add(NeedsLinkRetention+time.Minute)))
	assert.Equal(t, 0, count(), "a dismissed row is still pruned")
}
