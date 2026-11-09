package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aalejandrofer/grubdrops/internal/store"
)

func TestNeedsLink_Dismiss(t *testing.T) {
	e, nl := newNeedsLinkEnv(t)
	ctx := context.Background()
	drop := func(id, camp string) store.NeedsLinkDrop {
		return store.NeedsLinkDrop{Platform: "kick", Game: "Escape from Tarkov", CampaignID: camp, BenefitID: id, BenefitName: id, EndsAt: needsLinkNow}
	}
	require.NoError(t, nl.Sync(ctx, "b2", store.NeedsLinkSync{Refused: []store.NeedsLinkDrop{drop("d1", "c-day1"), drop("d1b", "c-day1"), drop("d3", "c-day3")}}, needsLinkNow))
	_, tok := e.session()
	ids := func() []string {
		_, b := e.do("GET", "/api/needs-link", "", nil)
		var got struct {
			Drops []struct {
				BenefitID string `json:"benefit_id"`
			} `json:"drops"`
		}
		require.NoError(t, json.Unmarshal(b, &got))
		out := []string{}
		for _, d := range got.Drops {
			out = append(out, d.BenefitID)
		}
		return out
	}
	group := map[string]string{"account_id": "b2", "campaign_id": "c-day1"}

	resp, b := e.do("POST", "/api/needs-link/dismiss", "", group)
	assert.Equal(t, 403, resp.StatusCode, "CSRF guards it")
	assert.JSONEq(t, `{"error":"csrf"}`, string(b))

	resp, b = e.do("POST", "/api/needs-link/dismiss", tok, group)
	require.Equal(t, 204, resp.StatusCode, string(b))
	assert.Equal(t, []string{"d3"}, ids())

	resp, _ = e.do("POST", "/api/needs-link/dismiss", tok, map[string]string{"account_id": "b2", "campaign_id": "c-none"})
	assert.Equal(t, 404, resp.StatusCode)
	resp, _ = e.do("POST", "/api/needs-link/dismiss", tok, map[string]string{"campaign_id": "c-day1"})
	assert.Equal(t, 400, resp.StatusCode, "account_id is required")
	resp, _ = e.do("POST", "/api/needs-link/dismiss", tok, map[string]any{"account_id": "b2", "campaign_id": "c-day1", "x": 1})
	assert.Equal(t, 400, resp.StatusCode)

	resp, _ = e.do("POST", "/api/needs-link/restore", tok, group)
	require.Equal(t, 204, resp.StatusCode)
	assert.ElementsMatch(t, []string{"d1", "d1b", "d3"}, ids())
}
