package kick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/aalejandrofer/grubdrops/internal/platform"
)

// challenge is one gamification challenge row: the daily reward is the one
// with recurrence "daily". Status turns "claimable" once the watch-time goal
// is met.
type challenge struct {
	ID, Name, Status, Recurrence string
}

// challenges lists the account's gamification challenges. AUTHED (bearer).
//
// Live shape (2026-11-02): {data:[{id, name, recurrence, status,
// condition:{progress, threshold, type}, window:{starts_at, ends_at},
// drop_table:[...]}], message}.
func (a *api) challenges(ctx context.Context, sess platform.Session) ([]challenge, error) {
	body, status, err := a.d.do(ctx, sess, http.MethodGet, dropsBase+"/api/v1/gamification/challenges", nil)
	if err != nil {
		return nil, err
	}
	if status != 200 {
		return nil, fmt.Errorf("challenges: status %d body %s", status, truncate(body, 200))
	}
	rows, err := asList(body, "data", "challenges")
	if err != nil {
		return nil, fmt.Errorf("decode challenges: %w", err)
	}
	out := make([]challenge, 0, len(rows))
	for _, m := range rows {
		out = append(out, challenge{
			ID:         mstr(m, "id"),
			Name:       mstr(m, "name", "title"),
			Status:     mstr(m, "status"),
			Recurrence: mstr(m, "recurrence"),
		})
	}
	return out, nil
}

// claimChallenge claims one challenge and returns the reward name and rarity
// when the response names them. AUTHED. POST with no body.
func (a *api) claimChallenge(ctx context.Context, sess platform.Session, id string) (reward, rarity string, err error) {
	path := dropsBase + "/api/v1/gamification/challenges/" + url.PathEscape(id) + "/claim"
	body, status, err := a.d.do(ctx, sess, http.MethodPost, path, nil)
	if err != nil {
		return "", "", err
	}
	if status != 200 && status != 201 {
		return "", "", fmt.Errorf("claim challenge: status %d body %s", status, truncate(body, 200))
	}
	// The response shape is not known yet: log it so claimedReward can be fit
	// to a real one.
	slog.Debug("kick daily reward claim response", "challenge", id, "status", status, "body", truncate(body, 1000))
	reward, rarity = claimedReward(body)
	return reward, rarity, nil
}

// claimedReward pulls a reward name and rarity from a claim response, whose
// shape is not known: it looks under data.reward, data.item, data and the top
// level. Both are empty when nothing matches.
func claimedReward(body []byte) (reward, rarity string) {
	var top map[string]any
	if json.Unmarshal(body, &top) != nil {
		return "", ""
	}
	var scopes []map[string]any
	if data, ok := top["data"].(map[string]any); ok {
		for _, k := range []string{"reward", "item", "prize", "drop"} {
			if sub, ok := data[k].(map[string]any); ok {
				scopes = append(scopes, sub)
			}
		}
		scopes = append(scopes, data)
	}
	scopes = append(scopes, top)
	for _, m := range scopes {
		if reward == "" {
			reward = mstr(m, "name", "title", "reward_name")
		}
		if rarity == "" {
			rarity = mstr(m, "rarity")
		}
	}
	return reward, rarity
}

// ClaimDailyRewards claims every daily challenge Kick marks claimable, except
// those skip reports true for. A failed claim does not stop the others: each
// failure is a platform.DailyRewardError in the joined error.
func (b *Backend) ClaimDailyRewards(ctx context.Context, s platform.Session, skip func(id string) bool) ([]platform.DailyReward, error) {
	list, err := b.api.challenges(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("kick daily reward: %w", err)
	}
	var (
		claimed []platform.DailyReward
		errs    []error
	)
	for _, c := range list {
		if c.ID == "" || c.Status != "claimable" || c.Recurrence != "daily" || (skip != nil && skip(c.ID)) {
			continue
		}
		reward, rarity, err := b.api.claimChallenge(ctx, s, c.ID)
		if err != nil {
			if ctx.Err() != nil {
				return claimed, ctx.Err()
			}
			errs = append(errs, &platform.DailyRewardError{ID: c.ID, Err: err})
			continue
		}
		if reward == c.Name {
			reward = "" // the response echoed the challenge, not the prize
		}
		claimed = append(claimed, platform.DailyReward{ID: c.ID, Name: c.Name, Reward: reward, Rarity: rarity})
	}
	return claimed, errors.Join(errs...)
}

var _ platform.DailyRewardClaimer = (*Backend)(nil)
