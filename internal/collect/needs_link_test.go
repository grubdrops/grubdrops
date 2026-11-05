package collect

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aalejandrofer/grubdrops/internal/plan"
	"github.com/aalejandrofer/grubdrops/internal/platform"
	"github.com/aalejandrofer/grubdrops/internal/runner"
	"github.com/aalejandrofer/grubdrops/internal/store"
)

func needsLinkRows(t *testing.T, c *Collector) map[string]store.NeedsLinkDrop {
	t.Helper()
	rows, err := c.NeedsLink.Unclaimed(context.Background())
	require.NoError(t, err)
	out := map[string]store.NeedsLinkDrop{}
	for _, r := range rows {
		out[r.BenefitID] = r
	}
	return out
}

func TestCollector_NeedsLinkOutlivesCampaign(t *testing.T) {
	c := newCollector(t)
	ctx := context.Background()
	done, half := ctarget("b1", 60, 60), ctarget("b2", 60, 30)
	done.Campaign.AccountLinkURL = "https://example.com/link"
	done.Campaign.EndsAt = cscanAt.Add(time.Hour)
	c.OnScan(ctx, runner.Scan{
		AccountID: "a1", Platform: "twitch", At: cscanAt, InventoryOK: true,
		Campaigns: []platform.Campaign{ccampaignOf(done, half)},
		Progress:  []platform.Progress{{BenefitID: "b1", MinutesWatched: 60}, {BenefitID: "b2", MinutesWatched: 30}},
		Result: plan.Result{Blocked: []plan.Blocked{
			{Target: done, Reason: plan.ReasonNotLinked},
			{Target: half, Reason: plan.ReasonNotLinked},
		}},
	})
	got := needsLinkRows(t, c)
	require.Len(t, got, 1, "only a completed drop counts")
	require.Equal(t, "https://example.com/link", got["b1"].LinkURL)
	require.Equal(t, "Camp", got["b1"].CampaignName)
	require.Equal(t, "twitch", got["b1"].Platform)

	// The campaign is gone from the next scan; the row stays.
	later := cscanAt.Add(2 * time.Hour)
	c.Now = func() time.Time { return later }
	c.OnScan(ctx, runner.Scan{AccountID: "a1", Platform: "twitch", At: later, InventoryOK: true})
	got = needsLinkRows(t, c)
	require.Len(t, got, 1)
	require.True(t, got["b1"].Expired(later), "ended and no longer listed")
}

func TestCollector_NeedsLinkClearsOnClaim(t *testing.T) {
	c := newCollector(t)
	ctx := context.Background()
	done := ctarget("b1", 60, 60)
	c.OnScan(ctx, runner.Scan{
		AccountID: "a1", Platform: "twitch", At: cscanAt,
		Result: plan.Result{Blocked: []plan.Blocked{{Target: done, Reason: plan.ReasonNotLinked}}},
	})
	require.Len(t, needsLinkRows(t, c), 1)
	require.NoError(t, c.RecordClaim(ctx, "a1", done, ""))
	require.Empty(t, needsLinkRows(t, c))

	// An inventory that shows the drop claimed clears it too.
	c.OnScan(ctx, runner.Scan{
		AccountID: "a1", Platform: "twitch", At: cscanAt,
		Result: plan.Result{Blocked: []plan.Blocked{{Target: ctarget("b3", 60, 60), Reason: plan.ReasonNotLinked}}},
	})
	require.Len(t, needsLinkRows(t, c), 1)
	c.OnScan(ctx, runner.Scan{
		AccountID: "a1", Platform: "twitch", At: cscanAt, InventoryOK: true,
		Progress: []platform.Progress{{BenefitID: "b3", MinutesWatched: 60, Claimed: true}},
	})
	require.Empty(t, needsLinkRows(t, c))
}

func TestCollector_NeedsLinkIgnoresFailedScan(t *testing.T) {
	c := newCollector(t)
	ctx := context.Background()
	c.OnScan(ctx, runner.Scan{
		AccountID: "a1", Platform: "twitch", At: cscanAt,
		Result: plan.Result{Blocked: []plan.Blocked{{Target: ctarget("b1", 60, 60), Reason: plan.ReasonNotLinked}}},
	})
	c.OnScan(ctx, runner.Scan{AccountID: "a1", Platform: "twitch", At: cscanAt, Err: context.DeadlineExceeded})
	got := needsLinkRows(t, c)
	require.Len(t, got, 1)
	require.True(t, got["b1"].Listed)
}
