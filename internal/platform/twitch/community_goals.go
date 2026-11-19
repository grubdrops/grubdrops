package twitch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/grubdrops/grubdrops/internal/platform"
)

// communityGoal is one goal in a ChannelPointsContext reply.
type communityGoal struct {
	ID                               string `json:"id"`
	Title                            string `json:"title"`
	Status                           string `json:"status"`
	IsInStock                        bool   `json:"isInStock"`
	PointsContributed                int    `json:"pointsContributed"`
	AmountNeeded                     int    `json:"amountNeeded"`
	PerStreamUserMaximumContribution int    `json:"perStreamUserMaximumContribution"`
}

// open reports whether the goal takes points now.
func (g communityGoal) open() bool {
	return g.Status == "STARTED" && g.IsInStock && g.ID != ""
}

// userPointsContribution is the UserPointsContribution reply.
type userPointsContribution struct {
	User *struct {
		Channel *struct {
			Self *struct {
				CommunityPoints *struct {
					GoalContributions []struct {
						Goal struct {
							ID string `json:"id"`
						} `json:"goal"`
						UserPointsContributedThisStream int `json:"userPointsContributedThisStream"`
					} `json:"goalContributions"`
				} `json:"communityPoints"`
			} `json:"self"`
		} `json:"channel"`
	} `json:"user"`
}

// contributeGoalResult is the ContributeCommunityPointsCommunityGoal reply.
type contributeGoalResult struct {
	Contribute *struct {
		Error *struct {
			Code string `json:"code"`
		} `json:"error"`
	} `json:"contributeCommunityPointsCommunityGoal"`
}

// ContributeCommunityGoals gives the account's points on channel (a login)
// to its running community goals. Each goal gets the least of what it still
// needs, what the account may still give it this stream, and the balance
// left. With no running goal it sends only the ChannelPointsContext query.
// Satisfies platform.GoalContributor.
func (b *Backend) ContributeCommunityGoals(ctx context.Context, s platform.Session, channel string) (platform.GoalContribution, error) {
	b.c.bind(s)
	return contributeCommunityGoals(ctx, b.c, s.AccessToken, channel)
}

var _ platform.GoalContributor = (*Backend)(nil)

func contributeCommunityGoals(ctx context.Context, c *client, token, channel string) (platform.GoalContribution, error) {
	var res platform.GoalContribution
	ch, err := channelPointsState(ctx, c, token, channel)
	if err != nil {
		return res, err
	}
	var goals []communityGoal
	if ch.CommunityPointsSettings != nil {
		for _, g := range ch.CommunityPointsSettings.Goals {
			if g.open() {
				goals = append(goals, g)
			}
		}
	}
	if len(goals) == 0 {
		return res, nil
	}
	balance := 0
	if ch.Self != nil && ch.Self.CommunityPoints != nil && ch.Self.CommunityPoints.Balance != nil {
		balance = *ch.Self.CommunityPoints.Balance
	}
	if balance <= 0 {
		return res, nil
	}
	if ch.ID == "" {
		return res, errors.New("community goals: no channel id")
	}
	var mine userPointsContribution
	if err := c.gql(ctx, token, OpUserPointsContribution, map[string]any{"channelLogin": channel}, &mine); err != nil {
		return res, fmt.Errorf("community goals: read contributions: %w", err)
	}
	given := map[string]int{}
	if u := mine.User; u != nil && u.Channel != nil && u.Channel.Self != nil && u.Channel.Self.CommunityPoints != nil {
		for _, gc := range u.Channel.Self.CommunityPoints.GoalContributions {
			given[gc.Goal.ID] = gc.UserPointsContributedThisStream
		}
	}
	var errs []error
	for _, g := range goals {
		amount := min(g.AmountNeeded-g.PointsContributed, g.PerStreamUserMaximumContribution-given[g.ID], balance)
		if amount <= 0 {
			continue
		}
		if err := contributeGoal(ctx, c, token, ch.ID, g.ID, amount); err != nil {
			errs = append(errs, fmt.Errorf("goal %q: %w", g.Title, err))
			continue
		}
		balance -= amount
		res.Goals++
		res.Points += amount
	}
	return res, errors.Join(errs...)
}

// contributeGoal gives amount points to one goal with one mutation.
func contributeGoal(ctx context.Context, c *client, token, channelID, goalID string, amount int) error {
	var tx [16]byte
	_, _ = rand.Read(tx[:])
	var out contributeGoalResult
	if err := c.gql(ctx, token, OpContributeCommunityPointsCommunityGoal, map[string]any{
		"input": map[string]any{
			"amount":        amount,
			"channelID":     channelID,
			"goalID":        goalID,
			"transactionID": hex.EncodeToString(tx[:]),
		},
	}, &out); err != nil {
		return fmt.Errorf("contribute: %w", err)
	}
	if out.Contribute == nil {
		return errors.New("contribute: no result")
	}
	if e := out.Contribute.Error; e != nil && e.Code != "" {
		return fmt.Errorf("contribute: refused: %s", e.Code)
	}
	return nil
}
