package kick

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grubdrops/grubdrops/internal/platform"
)

const challengesURL = "https://web.kick.com/api/v1/gamification/challenges"

// Only daily challenges Kick marks claimable are claimed; an in-progress or
// non-daily one is left alone. A refused claim is reported and does not stop
// the next one. The id is path-escaped.
func TestKickBackend_ClaimDailyRewards(t *testing.T) {
	f := &fakeDoer{resp: map[string]fakeResp{
		challengesURL: {200, `{"data":[
			{"id":"d-ok","name":"Daily Reward","recurrence":"daily","status":"claimable",
			 "condition":{"progress":60,"threshold":60,"type":"watch_time_minutes"}},
			{"id":"d-busy","name":"Daily Reward","recurrence":"daily","status":"in_progress"},
			{"id":"w-1","name":"Weekly","recurrence":"weekly","status":"claimable"},
			{"id":"d-bad","name":"Daily Reward","recurrence":"daily","status":"claimable"},
			{"id":"a/b c","name":"Daily Reward","recurrence":"daily","status":"claimable"}
		],"message":"success"}`},
		challengesURL + "/d-ok/claim":      {200, `{"data":{"reward":{"name":"Gold Badge","rarity":"rare"}},"message":"success"}`},
		challengesURL + "/d-bad/claim":     {422, `{"message":"already claimed"}`},
		challengesURL + "/a%2Fb%20c/claim": {201, `{"message":"success"}`},
	}}
	b := withFake(f)

	got, err := b.ClaimDailyRewards(context.Background(), sess("acc1"), nil)
	require.Error(t, err, "the refused claim is reported")
	var de *platform.DailyRewardError
	require.True(t, errors.As(err, &de))
	assert.Equal(t, "d-bad", de.ID)

	require.Len(t, got, 2, "the claim after the refused one still runs")
	assert.Equal(t, platform.DailyReward{ID: "d-ok", Name: "Daily Reward", Reward: "Gold Badge", Rarity: "rare"}, got[0])
	assert.Equal(t, platform.DailyReward{ID: "a/b c", Name: "Daily Reward"}, got[1], "no reward named: empty")

	var posts []string
	for _, c := range f.calls {
		if c.method == http.MethodPost {
			posts = append(posts, c.path)
			assert.Empty(t, c.body, "the claim has no body")
		}
	}
	assert.Equal(t, []string{
		challengesURL + "/d-ok/claim",
		challengesURL + "/d-bad/claim",
		challengesURL + "/a%2Fb%20c/claim",
	}, posts, "in_progress and weekly are not claimed")
}

// A skipped id is not claimed.
func TestKickBackend_ClaimDailyRewards_Skip(t *testing.T) {
	f := &fakeDoer{resp: map[string]fakeResp{
		challengesURL:               {200, `{"data":[{"id":"d1","name":"Daily Reward","recurrence":"daily","status":"claimable"}]}`},
		challengesURL + "/d1/claim": {200, `{}`},
	}}
	got, err := withFake(f).ClaimDailyRewards(context.Background(), sess("acc1"), func(id string) bool { return id == "d1" })
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Len(t, f.calls, 1, "list only, no claim")
}

// Nothing claimable: no claim and no error.
func TestKickBackend_ClaimDailyRewards_NoneReady(t *testing.T) {
	f := &fakeDoer{resp: map[string]fakeResp{
		challengesURL: {200, `{"data":[{"id":"d1","name":"Daily Reward","recurrence":"daily","status":"in_progress"}],"message":"success"}`},
	}}
	got, err := withFake(f).ClaimDailyRewards(context.Background(), sess("acc1"), nil)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Len(t, f.calls, 1)
}

// A failed list is an error with nothing claimed.
func TestKickBackend_ClaimDailyRewards_ListFails(t *testing.T) {
	f := &fakeDoer{resp: map[string]fakeResp{challengesURL: {401, `{}`}}}
	got, err := withFake(f).ClaimDailyRewards(context.Background(), sess("acc1"), nil)
	require.Error(t, err)
	assert.Empty(t, got)
}

// The claim response shape is unknown: the reward name and rarity are read
// from the likely spots, and a response echoing the challenge is not a prize.
func TestClaimedReward(t *testing.T) {
	for _, tc := range []struct{ body, reward, rarity string }{
		{`{"data":{"reward":{"name":"Gold","rarity":"epic"}}}`, "Gold", "epic"},
		{`{"data":{"item":{"title":"Emote"}},"rarity":"common"}`, "Emote", "common"},
		{`{"data":{"name":"Hat","rarity":"rare"}}`, "Hat", "rare"},
		{`{"message":"success"}`, "", ""},
		{`not json`, "", ""},
	} {
		reward, rarity := claimedReward([]byte(tc.body))
		assert.Equal(t, tc.reward, reward, tc.body)
		assert.Equal(t, tc.rarity, rarity, tc.body)
	}

	f := &fakeDoer{resp: map[string]fakeResp{
		challengesURL:               {200, `{"data":[{"id":"d1","name":"Daily Reward","recurrence":"daily","status":"claimable"}]}`},
		challengesURL + "/d1/claim": {200, `{"data":{"id":"d1","name":"Daily Reward","status":"claimed"}}`},
	}}
	got, err := withFake(f).ClaimDailyRewards(context.Background(), sess("acc1"), nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Empty(t, got[0].Reward, "the echoed challenge name is not the prize")
}
