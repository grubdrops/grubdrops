package twitch

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grubdrops/grubdrops/internal/platform"
)

// active on community-moments-channel-v1.<id> reaches OnMoment with the
// topic's channel and the moment ID. Other moment messages do not.
func TestPubSub_DispatchMomentActive(t *testing.T) {
	type call struct{ channel, moment string }
	var got []call
	p := NewPubSubClient("", PubSubHandlers{
		OnMoment: func(channelID, momentID string) { got = append(got, call{channelID, momentID}) },
	})
	p.dispatch("community-moments-channel-v1.123", []byte(`{"type":"active","data":{"moment_id":"m-1","channel_id":"123"}}`))
	p.dispatch("community-moments-channel-v1.123", []byte(`{"type":"inactive","data":{"moment_id":"m-1"}}`))
	p.dispatch("community-moments-channel-v1.123", []byte(`{"type":"active","data":{"moment_id":""}}`))
	p.dispatch("community-moments-channel-v1.123", []byte(`not json`))
	assert.Equal(t, []call{{"123", "m-1"}}, got)
	assert.Equal(t, "community-moments-channel-v1.123", TopicCommunityMoments("123"))
}

// The direct backend passes OnMoment into its PubSub handlers.
func TestBackend_MomentHookWired(t *testing.T) {
	b := &Backend{}
	var fired string
	b.SetAccountPubSubHooks("acct", platform.PubSubHooks{OnMoment: func(_, momentID string) { fired = momentID }})
	require.NotNil(t, b.pubsubHandlers.OnMoment)
	b.pubsubHandlers.OnMoment("1", "m-1")
	assert.Equal(t, "m-1", fired)
}

// SubscribeMoments adds the moments topic to the account's socket and
// UnsubscribeMoments removes it.
func TestBackend_SubscribeMomentsTopic(t *testing.T) {
	client := NewPubSubClient("", PubSubHandlers{})
	b := &Backend{pubsub: client}
	b.SubscribeMoments("acct", "123")
	client.mu.Lock()
	_, has := client.topics["community-moments-channel-v1.123"]
	n := len(client.topics)
	client.mu.Unlock()
	assert.True(t, has, "moments topic not added")
	assert.Equal(t, 1, n)

	b.UnsubscribeMoments("acct", "123")
	client.mu.Lock()
	_, has = client.topics["community-moments-channel-v1.123"]
	client.mu.Unlock()
	assert.False(t, has, "moments topic kept")
}

// ClaimMoment sends one CommunityMomentCallout_Claim mutation carrying the
// moment ID.
func TestBackend_ClaimMomentRequest(t *testing.T) {
	srv, seen := pointsServer(t, map[string]string{
		"CommunityMomentCallout_Claim": `{"data":{"claimCommunityMoment":{"moment":{"id":"m-1"},"error":null}}}`,
	})
	b := newForTest(srv.URL)
	require.NoError(t, b.ClaimMoment(context.Background(), platform.Session{AccessToken: "tok"}, "m-1"))
	reqs := seen()
	require.Len(t, reqs, 1)
	assert.Equal(t, "CommunityMomentCallout_Claim", reqs[0].Op)
	assert.Equal(t, OpCommunityMomentCallout_Claim.Hash, reqs[0].Hash)
	assert.Equal(t, map[string]any{"input": map[string]any{"momentID": "m-1"}}, reqs[0].Variables)
	assert.Empty(t, reqs[0].Header.Get("Client-Integrity"))

	require.Error(t, b.ClaimMoment(context.Background(), platform.Session{AccessToken: "tok"}, ""))
	assert.Len(t, seen(), 1, "empty moment ID must not send a request")
}

// A refused claim, which Twitch reports as a bare code in the payload's
// error field, is an error, not a claimed moment.
func TestBackend_ClaimMomentRefused(t *testing.T) {
	srv, _ := pointsServer(t, map[string]string{
		"CommunityMomentCallout_Claim": `{"data":{"claimCommunityMoment":{"moment":null,"error":"INVALID_MOMENT_ID"}}}`,
	})
	b := newForTest(srv.URL)
	err := b.ClaimMoment(context.Background(), platform.Session{AccessToken: "tok"}, "m-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_MOMENT_ID")
}

func TestGQLErrorCode(t *testing.T) {
	for raw, want := range map[string]string{
		``:                            "",
		`null`:                        "",
		`"INVALID_MOMENT_ID"`:         "INVALID_MOMENT_ID",
		`{"code":"FORBIDDEN"}`:        "FORBIDDEN",
		`{"message":"something odd"}`: `{"message":"something odd"}`,
	} {
		assert.Equal(t, want, gqlErrorCode(json.RawMessage(raw)), raw)
	}
}
