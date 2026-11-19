package twitch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grubdrops/grubdrops/internal/platform"
)

// A channel with no running goal costs only the context query.
func TestContributeCommunityGoals_NoOpenGoal(t *testing.T) {
	srv, seen := pointsServer(t, map[string]string{
		"ChannelPointsContext": `{"data":{"community":{"channel":{"id":"12345","communityPointsSettings":{"goals":[
			{"id":"g1","title":"Done","status":"ENDED","isInStock":true,"pointsContributed":10,"amountNeeded":10,"perStreamUserMaximumContribution":100},
			{"id":"g2","title":"Out","status":"STARTED","isInStock":false,"pointsContributed":0,"amountNeeded":10,"perStreamUserMaximumContribution":100}]},
			"self":{"communityPoints":{"balance":500}}}}}}`,
	})
	b := newForTest(srv.URL)
	got, err := b.ContributeCommunityGoals(context.Background(), platform.Session{AccessToken: "tok"}, "streamer")
	require.NoError(t, err)
	assert.Equal(t, platform.GoalContribution{}, got)
	assert.Len(t, seen(), 1)
}

// Each running goal gets the least of what it still needs, what the
// account may still give this stream, and the balance left.
func TestContributeCommunityGoals_Amounts(t *testing.T) {
	srv, seen := pointsServer(t, map[string]string{
		"ChannelPointsContext": `{"data":{"community":{"channel":{"id":"12345","communityPointsSettings":{"goals":[
			{"id":"g1","title":"Near done","status":"STARTED","isInStock":true,"pointsContributed":950,"amountNeeded":1000,"perStreamUserMaximumContribution":500},
			{"id":"g2","title":"Capped","status":"STARTED","isInStock":true,"pointsContributed":0,"amountNeeded":10000,"perStreamUserMaximumContribution":300},
			{"id":"g3","title":"Broke","status":"STARTED","isInStock":true,"pointsContributed":0,"amountNeeded":10000,"perStreamUserMaximumContribution":10000}]},
			"self":{"communityPoints":{"balance":400}}}}}}`,
		"UserPointsContribution": `{"data":{"user":{"channel":{"self":{"communityPoints":{"goalContributions":[
			{"goal":{"id":"g2"},"userPointsContributedThisStream":100}]}}}}}}`,
		"ContributeCommunityPointsCommunityGoal": `{"data":{"contributeCommunityPointsCommunityGoal":{"error":null}}}`,
	})
	b := newForTest(srv.URL)
	got, err := b.ContributeCommunityGoals(context.Background(), platform.Session{AccessToken: "tok"}, "streamer")
	require.NoError(t, err)
	// g1: needs 50. g2: may give 200 more. g3: the 150 left of 400.
	assert.Equal(t, platform.GoalContribution{Goals: 3, Points: 400}, got)
	reqs := seen()
	require.Len(t, reqs, 5)
	assert.Equal(t, "UserPointsContribution", reqs[1].Op)
	assert.Equal(t, OpUserPointsContribution.Hash, reqs[1].Hash)
	assert.Equal(t, map[string]any{"channelLogin": "streamer"}, reqs[1].Variables)
	want := []struct {
		goal   string
		amount float64
	}{{"g1", 50}, {"g2", 200}, {"g3", 150}}
	for i, w := range want {
		r := reqs[2+i]
		assert.Equal(t, "ContributeCommunityPointsCommunityGoal", r.Op)
		assert.Equal(t, OpContributeCommunityPointsCommunityGoal.Hash, r.Hash)
		in := r.Variables["input"].(map[string]any)
		assert.Equal(t, w.goal, in["goalID"])
		assert.Equal(t, w.amount, in["amount"])
		assert.Equal(t, "12345", in["channelID"])
		assert.Len(t, in["transactionID"], 32)
	}
}

// A refused contribution is an error and counts nothing.
func TestContributeCommunityGoals_Refused(t *testing.T) {
	srv, _ := pointsServer(t, map[string]string{
		"ChannelPointsContext": `{"data":{"community":{"channel":{"id":"12345","communityPointsSettings":{"goals":[
			{"id":"g1","title":"Goal","status":"STARTED","isInStock":true,"pointsContributed":0,"amountNeeded":1000,"perStreamUserMaximumContribution":100}]},
			"self":{"communityPoints":{"balance":400}}}}}}`,
		"UserPointsContribution":                 `{"data":{"user":{"channel":{"self":{"communityPoints":{"goalContributions":[]}}}}}}`,
		"ContributeCommunityPointsCommunityGoal": `{"data":{"contributeCommunityPointsCommunityGoal":{"error":{"code":"GOAL_NOT_ACTIVE"}}}}`,
	})
	b := newForTest(srv.URL)
	got, err := b.ContributeCommunityGoals(context.Background(), platform.Session{AccessToken: "tok"}, "streamer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GOAL_NOT_ACTIVE")
	assert.Equal(t, platform.GoalContribution{}, got)
}

// No balance means no contribution and no further request.
func TestContributeCommunityGoals_NoBalance(t *testing.T) {
	srv, seen := pointsServer(t, map[string]string{
		"ChannelPointsContext": `{"data":{"community":{"channel":{"id":"12345","communityPointsSettings":{"goals":[
			{"id":"g1","title":"Goal","status":"STARTED","isInStock":true,"pointsContributed":0,"amountNeeded":1000,"perStreamUserMaximumContribution":100}]},
			"self":{"communityPoints":{"balance":0}}}}}}`,
	})
	b := newForTest(srv.URL)
	got, err := b.ContributeCommunityGoals(context.Background(), platform.Session{AccessToken: "tok"}, "streamer")
	require.NoError(t, err)
	assert.Equal(t, platform.GoalContribution{}, got)
	assert.Len(t, seen(), 1)
}
