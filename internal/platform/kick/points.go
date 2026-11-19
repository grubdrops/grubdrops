package kick

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/grubdrops/grubdrops/internal/platform"
)

// channelPointsResp is the /api/v2/channels/{slug}/points reply, the read
// the web player's channel points button makes.
type channelPointsResp struct {
	Data *struct {
		Points *int `json:"points"`
	} `json:"data"`
}

// ChannelPoints reads the account's channel points on channel slug. AUTHED.
// A channel without channel points answers 404 or no points, reported as
// platform.ErrNoPointsBalance.
func (a *api) ChannelPoints(ctx context.Context, sess platform.Session, slug string) (int, error) {
	body, status, err := a.d.do(ctx, sess, http.MethodGet, discoveryBase+"/api/v2/channels/"+url.PathEscape(slug)+"/points", nil)
	if err != nil {
		return 0, err
	}
	switch status {
	case http.StatusOK:
	case http.StatusNotFound:
		return 0, platform.ErrNoPointsBalance
	default:
		return 0, &statusError{what: "channel points " + slug, status: status}
	}
	var resp channelPointsResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("decode channel points: %w", err)
	}
	if resp.Data == nil || resp.Data.Points == nil {
		return 0, platform.ErrNoPointsBalance
	}
	return *resp.Data.Points, nil
}

// ChannelPointsBalance reads the account's channel points on channel (a
// slug). Kick has no bonus to claim, and its points come from chat activity
// rather than watch time, so this only reads. Satisfies
// platform.PointsBalancer.
func (b *Backend) ChannelPointsBalance(ctx context.Context, s platform.Session, channel string) (int, error) {
	if channel == "" {
		return 0, fmt.Errorf("kick channel points: channel is required")
	}
	return b.api.ChannelPoints(ctx, s, channel)
}

var _ platform.PointsBalancer = (*Backend)(nil)
