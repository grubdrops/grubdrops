package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grubdrops/grubdrops/internal/platform"
)

func fixtureRewardCampaign(t *testing.T) rgCampaign {
	t.Helper()
	var env struct {
		Data rgCampaignsData `json:"data"`
	}
	require.NoError(t, json.Unmarshal(loadFixture(t, "channel_drop_campaigns.json"), &env))
	require.Len(t, env.Data.ChannelDropCampaigns, 1)
	return env.Data.ChannelDropCampaigns[0]
}

func TestMapRewardCampaign_DailyChest(t *testing.T) {
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	c, ok := mapRewardCampaign(fixtureRewardCampaign(t), "gta", now)
	require.True(t, ok)
	assert.Equal(t, platform.KindRepeat, c.Kind)
	assert.Equal(t, "Grand Theft Auto VI", c.Game)
	assert.Equal(t, "1867279146", c.GameID)
	assert.Equal(t, "active", c.Status)
	assert.True(t, c.AccountLinked)
	require.Len(t, c.Benefits, 6, "one benefit per day of the watch group, no sub or ultimate rows")
	gid := "2283245e-d15c-11f1-8cdd-0a58a9feac02"
	for i, b := range c.Benefits {
		assert.Equal(t, repeatBenefitID(gid, i+1), b.ID)
		assert.Equal(t, 60, b.RequiredMinutes)
		assert.Empty(t, b.RewardID, "a synthetic day is never claimed elsewhere")
		require.NotNil(t, b.Repeat)
		assert.Equal(t, i+1, b.Repeat.Day)
		assert.Equal(t, 6, b.Repeat.Days)
		assert.Equal(t, 24*time.Hour, b.Repeat.Reset)
		assert.Equal(t, []string{"1867279146"}, b.Repeat.CategoryIDs)
		if i == 0 {
			assert.Empty(t, b.Preconditions)
		} else {
			assert.Equal(t, []string{repeatBenefitID(gid, i)}, b.Preconditions)
		}
	}
	assert.Equal(t, "2283245e-d15c-11f1-8cdd-0a58a9feac02:day01", c.Benefits[0].ID)
}

func TestMapRewardCampaign_SubOnlyIsNotMapped(t *testing.T) {
	rc := fixtureRewardCampaign(t)
	var subs []rgGroup
	for _, g := range rc.RewardGroups {
		if g.ProgressCriteria.RequirementType == "SUB" {
			subs = append(subs, g)
		}
	}
	require.NotEmpty(t, subs)
	rc.RewardGroups = subs
	_, ok := mapRewardCampaign(rc, "gta", time.Now())
	assert.False(t, ok)
}

func progressCamp(self rgSelf) rgCampaign {
	rc := rgCampaign{ID: "camp"}
	g := rgGroup{ID: "grp", Self: &self}
	g.ProgressCriteria.RequirementType = "WATCH"
	g.ProgressCriteria.Requirements.MinutesWatched = 60
	g.ProgressCriteria.IsRepeatable = true
	g.ProgressCriteria.IsAutoProgressable = true
	g.ProgressCriteria.RepeatableConfig = &struct {
		RepeatableTimes      int   `json:"repeatableTimes"`
		ResetIntervalSeconds int64 `json:"resetIntervalSeconds"`
	}{6, 86400}
	rc.RewardGroups = []rgGroup{g}
	return rc
}

func TestProgressRows(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour).Format(time.RFC3339)
	future := now.Add(time.Hour).Format(time.RFC3339)

	win := &struct {
		StartedAt string `json:"startedAt"`
		ExpiresAt string `json:"expiresAt"`
	}{StartedAt: past, ExpiresAt: future}
	rows := progressRows(progressCamp(rgSelf{Status: "IN_PROGRESS", CurrentMinutesWatched: 20, GrantCount: 2, NextEligibleAt: past, CurrentWindow: win}), "u1", now)
	require.Len(t, rows, 3)
	assert.Equal(t, platform.Progress{BenefitID: "grp:day01", MinutesWatched: 60, Claimed: true}, rows[0])
	assert.True(t, rows[1].Claimed)
	assert.Equal(t, "grp:day03", rows[2].BenefitID)
	assert.Equal(t, 20, rows[2].MinutesWatched)
	assert.False(t, rows[2].Claimed)
	assert.False(t, rows[2].NextEligibleAt.IsZero())

	rows = progressRows(progressCamp(rgSelf{Status: "CLAIMABLE", CurrentMinutesWatched: 10, GrantCount: 0}), "u1", now)
	require.Len(t, rows, 1)
	assert.Equal(t, 60, rows[0].MinutesWatched)
	assert.Equal(t, "u1#camp#grp", rows[0].InstanceID)

	rows = progressRows(progressCamp(rgSelf{Status: "CLAIMED", GrantCount: 6}), "u1", now)
	require.Len(t, rows, 6)
	for _, r := range rows {
		assert.True(t, r.Claimed)
	}

	rows = progressRows(progressCamp(rgSelf{Status: "IN_PROGRESS", CurrentMinutesWatched: 20, GrantCount: 2, NextEligibleAt: future}), "u1", now)
	require.Len(t, rows, 3)
	assert.Equal(t, 0, rows[2].MinutesWatched, "before the next window nothing counts")

	// Minutes of the day before a grant do not carry over: no window, or a
	// window that opened before the next eligible time.
	rows = progressRows(progressCamp(rgSelf{Status: "IN_PROGRESS", CurrentMinutesWatched: 60, GrantCount: 1, NextEligibleAt: past}), "u1", now)
	assert.Equal(t, 0, rows[1].MinutesWatched, "no window after a grant")
	old := &struct {
		StartedAt string `json:"startedAt"`
		ExpiresAt string `json:"expiresAt"`
	}{StartedAt: now.Add(-3 * time.Hour).Format(time.RFC3339), ExpiresAt: future}
	rows = progressRows(progressCamp(rgSelf{Status: "IN_PROGRESS", CurrentMinutesWatched: 60, GrantCount: 1, NextEligibleAt: past, CurrentWindow: old}), "u1", now)
	assert.Equal(t, 0, rows[1].MinutesWatched, "window opened before the next eligible time")

	// An unknown user id leaves the instance id empty.
	rows = progressRows(progressCamp(rgSelf{Status: "CLAIMABLE"}), "", now)
	assert.Empty(t, rows[0].InstanceID)
	assert.Equal(t, 60, rows[0].MinutesWatched)
}

func TestMergeShared_SkipsRepeatKind(t *testing.T) {
	c := androidCamp("campR", "Rust")
	c.Kind = platform.KindRepeat
	assert.Empty(t, mergeShared(nil, []platform.Campaign{c}, nil, time.Now()))
}

// rgServer answers the reward group flow. progress is the body for the
// progress query.
func rgServer(t *testing.T, progress string, progressCalls, claimCalls *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			OperationName string `json:"operationName"`
		}
		require.NoError(t, json.Unmarshal(raw, &req))
		assert.Empty(t, r.Header.Get("Client-Integrity"))
		switch req.OperationName {
		case "GameSlug":
			_, _ = w.Write([]byte(`{"data":{"game":{"slug":"grand-theft-auto-vi"}}}`))
		case OpGameDirectory.Name:
			_, _ = w.Write([]byte(`{"data":{"game":{"streams":{"edges":[{"node":{"id":"s1","viewersCount":9,"broadcaster":{"id":"90075649","login":"gta"},"game":{"id":"1867279146","displayName":"Grand Theft Auto VI"}}}]}}}}`))
		case "ChannelDropsCampaigns":
			_, _ = w.Write(loadFixture(t, "channel_drop_campaigns.json"))
		case "DropChannelCampaignsProgress":
			progressCalls.Add(1)
			_, _ = w.Write([]byte(progress))
		case OpClaimDrop.Name:
			claimCalls.Add(1)
			_, _ = w.Write(loadFixture(t, "claim_ok.json"))
		case "CurrentUser":
			_, _ = w.Write([]byte(`{"data":{"currentUser":{"id":"42"}}}`))
		case OpInventory.Name:
			_, _ = w.Write([]byte(`{"data":{"currentUser":{"inventory":{"dropCampaignsInProgress":[]}}}}`))
		default:
			_, _ = w.Write([]byte(`{"data":{}}`))
		}
	}))
}

func TestRewardGroupCampaigns_Discovery(t *testing.T) {
	var pc, cc atomic.Int32
	srv := rgServer(t, `{"data":{}}`, &pc, &cc)
	defer srv.Close()
	b := newForTest(srv.URL)
	sess := platform.Session{AccessToken: "tok", Games: []string{"Grand Theft Auto VI", "grand-theft-auto-vi"}}

	got := b.disc.rewardGroupCampaigns(context.Background(), sess, b.chans, map[string]bool{})
	require.Len(t, got, 1, "the game under two names is read once")
	assert.Equal(t, platform.KindRepeat, got[0].Kind)

	// A campaign the dashboard already lists is skipped.
	got = b.disc.rewardGroupCampaigns(context.Background(), sess, b.chans, map[string]bool{got[0].ID: true})
	assert.Empty(t, got)

	// The game filter has the last word.
	sess.GameFilter = func(string) bool { return false }
	assert.Empty(t, b.disc.rewardGroupCampaigns(context.Background(), sess, b.chans, map[string]bool{}))
}

func TestInventoryProgress_IntegrityBlockedProgressIsSwallowed(t *testing.T) {
	var pc, cc atomic.Int32
	srv := rgServer(t, `{"errors":[{"message":"failed integrity check"}]}`, &pc, &cc)
	defer srv.Close()
	b := newForTest(srv.URL)
	sess := platform.Session{AccessToken: "tok", Games: []string{"Grand Theft Auto VI"}}
	ctx := context.Background()

	camps, err := b.ListActiveCampaigns(ctx, sess)
	require.NoError(t, err)
	require.NotEmpty(t, camps)

	rows, err := b.InventoryProgress(ctx, sess)
	require.NoError(t, err, "an integrity-blocked progress query must not fail the inventory read")
	assert.Empty(t, rows)
	assert.Equal(t, int32(1), pc.Load())

	// A later poll after the cache expires still makes no second request.
	b.disc.mu.Lock()
	b.disc.rgProgAt = time.Time{}
	b.disc.mu.Unlock()
	_, err = b.InventoryProgress(ctx, sess)
	require.NoError(t, err)
	assert.Equal(t, int32(1), pc.Load(), "the progress query rests after a block")
}

func TestInventoryProgress_AppendsRewardGroupRows(t *testing.T) {
	var pc, cc atomic.Int32
	body, err := json.Marshal(map[string]any{"data": map[string]any{"channelDropCampaignsProgress": []any{
		progressCamp(rgSelf{Status: "CLAIMABLE", GrantCount: 1}),
	}}})
	require.NoError(t, err)
	srv := rgServer(t, string(body), &pc, &cc)
	defer srv.Close()
	b := newForTest(srv.URL)
	sess := platform.Session{AccessToken: "tok", Games: []string{"Grand Theft Auto VI"}}
	ctx := context.Background()
	_, err = b.ListActiveCampaigns(ctx, sess)
	require.NoError(t, err)
	rows, err := b.InventoryProgress(ctx, sess)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.True(t, rows[0].Claimed)
	assert.Equal(t, "42#camp#grp", rows[1].InstanceID)
}

func TestClaimRepeat(t *testing.T) {
	var pc, cc atomic.Int32
	srv := rgServer(t, `{"data":{}}`, &pc, &cc)
	defer srv.Close()
	b := newForTest(srv.URL)
	sess := platform.Session{AccessToken: "tok"}
	ctx := context.Background()
	drop := platform.DropBenefit{ID: "grp:day01", CampaignID: "camp", Repeat: &platform.Repeat{Day: 1, Days: 6}}

	require.NoError(t, b.Claim(ctx, sess, drop), "auto-granted day")
	assert.Equal(t, int32(0), cc.Load(), "no request for an auto-granted day")

	drop.InstanceID = "42#camp#grp"
	require.NoError(t, b.Claim(ctx, sess, drop))
	assert.Equal(t, int32(1), cc.Load(), "one claim, no inventory read")
}

func TestListEligibleChannels_RepeatRetriesUnfiltered(t *testing.T) {
	var filters []int
	srv := fakeGQL(t, map[string]func(map[string]any) string{
		"GameSlug": func(map[string]any) string { return `{"data":{"game":{"slug":"gta"}}}` },
		"DirectoryPage_Game": func(v map[string]any) string {
			opts, _ := v["options"].(map[string]any)
			f, _ := opts["systemFilters"].([]any)
			filters = append(filters, len(f))
			if len(f) > 0 {
				return `{"data":{"game":{"streams":{"edges":[]}}}}`
			}
			return `{"data":{"game":{"streams":{"edges":[{"node":{"id":"s1","viewersCount":1,"broadcaster":{"id":"1","login":"a"}}}]}}}}`
		},
	})
	defer srv.Close()
	b := newForTest(srv.URL)
	sess := platform.Session{AccessToken: "tok"}

	streams, err := b.ListEligibleChannels(context.Background(), sess, platform.Campaign{ID: "r", Game: "GTA", Kind: platform.KindRepeat})
	require.NoError(t, err)
	assert.Len(t, streams, 1)
	assert.Equal(t, []int{1, 0}, filters)

	filters = nil
	streams, err = b.ListEligibleChannels(context.Background(), sess, platform.Campaign{ID: "d", Game: "GTA", Kind: "drop"})
	require.NoError(t, err)
	assert.Empty(t, streams, "a normal drop keeps the drops-enabled filter")
	assert.Equal(t, []int{1}, filters)
}

func TestClaimRepeat_RefusedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"claimDropRewards":{"status":"INVALID_DROP_INSTANCE"}}}`))
	}))
	defer srv.Close()
	cl := &claimer{c: newTestClient(srv.URL)}
	err := cl.claimRepeat(context.Background(), platform.Session{AccessToken: "t"},
		platform.DropBenefit{ID: "g:day01", InstanceID: "u#c#g", Repeat: &platform.Repeat{}})
	require.ErrorIs(t, err, platform.ErrClaimRefused)
}

func TestRewardGroupProgress_KeepsLastGoodRowsOnFailure(t *testing.T) {
	var pc, cc atomic.Int32
	body, err := json.Marshal(map[string]any{"data": map[string]any{"channelDropCampaignsProgress": []any{
		progressCamp(rgSelf{Status: "IN_PROGRESS", GrantCount: 1}),
	}}})
	require.NoError(t, err)
	fail := atomic.Bool{}
	inner := rgServer(t, string(body), &pc, &cc)
	defer inner.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if fail.Load() && json.Valid(raw) && containsOp(raw, "DropChannelCampaignsProgress") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		req, _ := http.NewRequest(r.Method, inner.URL, bytesReader(raw))
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		_, _ = w.Write(b)
	}))
	defer srv.Close()
	b := newForTest(srv.URL)
	sess := platform.Session{AccessToken: "tok", Games: []string{"Grand Theft Auto VI"}}
	ctx := context.Background()
	_, err = b.ListActiveCampaigns(ctx, sess)
	require.NoError(t, err)
	rows, err := b.InventoryProgress(ctx, sess)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	fail.Store(true)
	b.disc.mu.Lock()
	b.disc.rgProgAt = time.Time{}
	b.disc.mu.Unlock()
	rows, err = b.InventoryProgress(ctx, sess)
	require.NoError(t, err)
	assert.Len(t, rows, 2, "a failed pass keeps the last good rows")
}

func TestRewardGroups_EndedCampaignStopsProgressPolling(t *testing.T) {
	var pc, cc atomic.Int32
	srv := rgServer(t, `{"data":{}}`, &pc, &cc)
	defer srv.Close()
	b := newForTest(srv.URL)
	sess := platform.Session{AccessToken: "tok", Games: []string{"Grand Theft Auto VI"}}
	_, err := b.ListActiveCampaigns(context.Background(), sess)
	require.NoError(t, err)
	assert.True(t, b.disc.hasRewardGroups())

	b.disc.mu.Lock()
	for k, e := range b.disc.rgByGame {
		for i := range e.camps {
			e.camps[i].EndsAt = time.Now().Add(-time.Minute)
		}
		b.disc.rgByGame[k] = e
	}
	b.disc.mu.Unlock()
	assert.False(t, b.disc.hasRewardGroups())
}

func TestRewardGroups_FailureIsCached(t *testing.T) {
	var campCalls atomic.Int32
	srv := fakeGQL(t, map[string]func(map[string]any) string{
		"GameSlug":           func(map[string]any) string { return `{"data":{"game":{"slug":"gta"}}}` },
		"DirectoryPage_Game": func(map[string]any) string { return dirRust },
		"ChannelDropsCampaigns": func(map[string]any) string {
			campCalls.Add(1)
			return `{"errors":[{"message":"boom"}]}`
		},
	})
	defer srv.Close()
	b := newForTest(srv.URL)
	sess := platform.Session{AccessToken: "tok", Games: []string{"GTA"}}
	for i := 0; i < 3; i++ {
		assert.Empty(t, b.disc.rewardGroupCampaigns(context.Background(), sess, b.chans, map[string]bool{}))
	}
	assert.Equal(t, int32(1), campCalls.Load(), "a failed read is not retried on every scan")
}

func containsOp(raw []byte, op string) bool { return bytes.Contains(raw, []byte(`"`+op+`"`)) }

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

func TestRewardGroups_CanceledPassIsNotCached(t *testing.T) {
	var campCalls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	srv := fakeGQL(t, map[string]func(map[string]any) string{
		"GameSlug":           func(map[string]any) string { return `{"data":{"game":{"slug":"gta"}}}` },
		"DirectoryPage_Game": func(map[string]any) string { return dirRust },
		"ChannelDropsCampaigns": func(map[string]any) string {
			if campCalls.Add(1) == 1 {
				cancel() // the scan is canceled while this read is in flight
				return `{"errors":[{"message":"canceled"}]}`
			}
			return `{"data":{"channelDropCampaigns":[]}}`
		},
	})
	defer srv.Close()
	b := newForTest(srv.URL)
	sess := platform.Session{AccessToken: "tok", Games: []string{"GTA"}}
	assert.Empty(t, b.disc.rewardGroupCampaigns(ctx, sess, b.chans, map[string]bool{}))
	b.disc.mu.Lock()
	n := len(b.disc.rgByGame)
	b.disc.mu.Unlock()
	assert.Zero(t, n, "a canceled pass caches nothing")
	b.disc.rewardGroupCampaigns(context.Background(), sess, b.chans, map[string]bool{})
	assert.Equal(t, int32(2), campCalls.Load(), "the next pass reads the game")
}

func TestRewardGroups_OneOffWatchGroupIsNotMapped(t *testing.T) {
	// Live Twitch serves ordinary drop campaigns as one-off watch groups too,
	// reported CLAIMED with grantCount 0; the dashboard path mines those.
	rc := progressCamp(rgSelf{Status: "CLAIMED", CurrentMinutesWatched: 60, GrantCount: 0})
	rc.RewardGroups[0].ProgressCriteria.IsRepeatable = false
	rc.RewardGroups[0].ProgressCriteria.RepeatableConfig = nil
	_, ok := mapRewardCampaign(rc, "tarkov", time.Now())
	assert.False(t, ok)
	assert.Empty(t, progressRows(rc, "u1", time.Now()))
}

func TestProgressRows_FulfilledClaimsEveryDay(t *testing.T) {
	rows := progressRows(progressCamp(rgSelf{Status: "FULFILLED", GrantCount: 0}), "u1", time.Now())
	require.Len(t, rows, 6)
	for _, r := range rows {
		assert.True(t, r.Claimed)
	}
}

func TestRewardGroups_PollingOutlivesTheDiscoveryCache(t *testing.T) {
	// Seen live: scans refresh the cache about hourly, so progress polling
	// stopped 30 minutes in and the day restarted from a local zero.
	var pc, cc atomic.Int32
	srv := rgServer(t, `{"data":{}}`, &pc, &cc)
	defer srv.Close()
	b := newForTest(srv.URL)
	sess := platform.Session{AccessToken: "tok", Games: []string{"Grand Theft Auto VI"}}
	_, err := b.ListActiveCampaigns(context.Background(), sess)
	require.NoError(t, err)

	age := func(d time.Duration) {
		b.disc.mu.Lock()
		for k, e := range b.disc.rgByGame {
			e.at = time.Now().Add(-d)
			b.disc.rgByGame[k] = e
		}
		b.disc.mu.Unlock()
	}
	age(rgTTL + 5*time.Minute)
	assert.True(t, b.disc.hasRewardGroups(), "an hour-old entry still polls")
	age(rgPollStale + time.Minute)
	assert.False(t, b.disc.hasRewardGroups(), "a game nobody rescanned for hours stops")
}

func TestProgressRows_GrantedWindowGatesNextDay(t *testing.T) {
	// Live self right after day 1 was granted: the window's 60 minutes stay,
	// nextEligibleAt is empty, and the window runs on.
	now := time.Date(2026, 11, 19, 1, 14, 0, 0, time.UTC)
	exp := time.Date(2026, 11, 19, 22, 31, 38, 0, time.UTC)
	win := &struct {
		StartedAt string `json:"startedAt"`
		ExpiresAt string `json:"expiresAt"`
	}{StartedAt: "2026-11-18T22:31:38Z", ExpiresAt: exp.Format(time.RFC3339)}
	rows := progressRows(progressCamp(rgSelf{Status: "IN_PROGRESS", CurrentMinutesWatched: 60, GrantCount: 1, CurrentWindow: win}), "u1", now)
	require.Len(t, rows, 2)
	assert.True(t, rows[0].Claimed)
	assert.Equal(t, "grp:day02", rows[1].BenefitID)
	assert.Equal(t, 0, rows[1].MinutesWatched, "the granted window's minutes are not day 2's")
	assert.True(t, rows[1].NextEligibleAt.Equal(exp), "day 2 opens when the window expires")

	// A fresh window after expiry counts normally.
	fresh := &struct {
		StartedAt string `json:"startedAt"`
		ExpiresAt string `json:"expiresAt"`
	}{StartedAt: exp.Add(time.Hour).Format(time.RFC3339), ExpiresAt: exp.Add(25 * time.Hour).Format(time.RFC3339)}
	rows = progressRows(progressCamp(rgSelf{Status: "IN_PROGRESS", CurrentMinutesWatched: 12, GrantCount: 1, CurrentWindow: fresh}), "u1", exp.Add(90*time.Minute))
	assert.Equal(t, 12, rows[1].MinutesWatched)
	assert.True(t, rows[1].NextEligibleAt.IsZero())
}
