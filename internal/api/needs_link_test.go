package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aalejandrofer/grubdrops/internal/store"
)

var needsLinkNow = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func newNeedsLinkEnv(t *testing.T) (*resEnv, *store.NeedsLinkDrops) {
	t.Helper()
	var nl *store.NeedsLinkDrops
	e := newResEnv(t, func(d *Deps) {
		nl = store.NewNeedsLinkDrops(d.DB, d.Q)
		d.NeedsLink = nl
		d.Now = func() time.Time { return needsLinkNow }
	})
	e.s.routesNeedsLink(e.s.protected)
	return e, nl
}

func TestNeedsLink_List(t *testing.T) {
	e, nl := newNeedsLinkEnv(t)
	ctx := context.Background()
	ended := needsLinkNow.Add(-24 * time.Hour)
	drop := func(id string) store.NeedsLinkDrop {
		return store.NeedsLinkDrop{
			Platform: "kick", Game: "Counter-Strike 2", CampaignID: "c-iem", CampaignName: "IEM Beijing 2026 - Day 2",
			BenefitID: id, BenefitName: "Sticker " + id, ImageURL: "https://files.kick.com/" + id + ".png",
			LinkURL: "https://example.com/connect", EndsAt: ended,
		}
	}
	require.NoError(t, nl.Sync(ctx, "b2", store.NeedsLinkSync{Refused: []store.NeedsLinkDrop{drop("r1"), drop("r2"), drop("r3")}}, ended.Add(-time.Hour)))
	// After the end the inventory still lists r1; r2 dropped off it.
	require.NoError(t, nl.Sync(ctx, "b2", store.NeedsLinkSync{Inventory: true, Listed: map[string]bool{"r1": true, "r3": true}}, needsLinkNow))
	// r3 was claimed.
	require.NoError(t, store.NewClaimLog(e.q).Record(ctx, store.ClaimEntry{AccountID: "b2", BenefitID: "r3", Platform: "kick", Source: store.ClaimSourceGrubDrops, ClaimedAt: needsLinkNow}))

	resp, b := e.do("GET", "/api/needs-link", "", nil)
	require.Equal(t, 200, resp.StatusCode, string(b))
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	var got struct {
		Drops []map[string]any `json:"drops"`
	}
	require.NoError(t, json.Unmarshal(b, &got))
	require.Len(t, got.Drops, 2, "the claimed drop is left out")
	byID := map[string]map[string]any{}
	for _, d := range got.Drops {
		byID[d["benefit_id"].(string)] = d
	}
	r1 := byID["r1"]
	assert.Equal(t, "b2", r1["account_id"])
	assert.Equal(t, "Bob", r1["account_name"])
	assert.Equal(t, "kick", r1["platform"])
	assert.Equal(t, "Sticker r1", r1["benefit_name"])
	assert.Equal(t, "https://files.kick.com/r1.png", r1["image_url"])
	assert.Equal(t, "c-iem", r1["campaign_id"])
	assert.Equal(t, "IEM Beijing 2026 - Day 2", r1["campaign_name"])
	assert.Equal(t, "Counter-Strike 2", r1["game"])
	assert.Equal(t, "https://example.com/connect", r1["link_url"])
	assert.Equal(t, "2026-11-02T12:00:00Z", r1["ends_at"])
	assert.Equal(t, false, r1["expired"], "still listed: the claim can land")
	assert.Equal(t, true, byID["r2"]["expired"], "ended and gone from the inventory")
}

func TestNeedsLink_EmptyIsList(t *testing.T) {
	e, _ := newNeedsLinkEnv(t)
	resp, b := e.do("GET", "/api/needs-link", "", nil)
	require.Equal(t, 200, resp.StatusCode)
	assert.JSONEq(t, `{"drops":[]}`, string(b))
}

func TestNeedsLink_Unavailable(t *testing.T) {
	e := newResEnv(t, nil)
	e.s.routesNeedsLink(e.s.protected)
	resp, _ := e.do("GET", "/api/needs-link", "", nil)
	assert.Equal(t, 503, resp.StatusCode)
}
