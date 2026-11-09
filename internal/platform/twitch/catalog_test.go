package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grubdrops/grubdrops/internal/platform"
)

func TestCatalog_ActiveWithRewards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req gqlRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.OperationName {
		case OpCampaigns.Name:
			_, _ = w.Write(loadFixture(t, "campaigns.json"))
		case OpDropCampaignDetails.Name:
			assert.Equal(t, "42", req.Variables["channelLogin"])
			_, _ = w.Write(loadFixture(t, "campaign_details.json"))
		default:
			t.Fatalf("unexpected op %q", req.OperationName)
		}
	}))
	defer srv.Close()

	b := newForTest(srv.URL)
	camps, err := b.Catalog(context.Background(), platform.Session{AccessToken: "tok", UserID: "42"})
	require.NoError(t, err)
	require.Len(t, camps, 1)
	assert.Equal(t, "camp1", camps[0].ID)
	assert.Equal(t, "Rust", camps[0].Game)
	assert.Equal(t, "https://static-cdn.jtvnw.net/ttv-boxart/263490-144x192.jpg", camps[0].BoxArtURL, "gql game.boxArtURL, size filled")
	require.Len(t, camps[0].Benefits, 1)
	assert.Equal(t, "Wolf Helmet", camps[0].Benefits[0].Name)
	assert.Equal(t, 60, camps[0].Benefits[0].RequiredMinutes)
	assert.Equal(t, []string{"fakestreamer", "another"}, camps[0].AllowedChannels, "allow-list rides on the campaign")
	assert.Equal(t, 2, camps[0].AllowedChannelCount)
	assert.Empty(t, b.disc.drainAllowed(), "catalog must not capture allow-lists")
}

func TestCatalog_NullCurrentUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"currentUser":null}}`))
	}))
	defer srv.Close()

	_, err := newForTest(srv.URL).Catalog(context.Background(), platform.Session{AccessToken: "tok", UserID: "1"})
	assert.True(t, errors.Is(err, ErrCatalogUnavailable))
}

func TestCatalog_TVTokenRefused(t *testing.T) {
	_, err := newForTest("http://127.0.0.1:1").Catalog(context.Background(), platform.Session{AccessToken: "tok", ClientID: ClientTV})
	assert.True(t, errors.Is(err, ErrCatalogUnavailable))
}

func TestCatalog_DetailFailureKeepsRow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req gqlRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		if req.OperationName == OpCampaigns.Name {
			_, _ = w.Write(loadFixture(t, "campaigns.json"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	camps, err := newForTest(srv.URL).Catalog(context.Background(), platform.Session{AccessToken: "tok", UserID: "1"})
	require.NoError(t, err)
	require.Len(t, camps, 1)
	assert.Empty(t, camps[0].Benefits)
}

// Upcoming campaigns stay in the catalog, flagged, with their drops from the
// same DropCampaignDetails query; expired ones are still left out.
func TestCatalog_KeepsUpcomingWithDrops(t *testing.T) {
	var (
		mu      sync.Mutex
		details []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req gqlRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.OperationName {
		case OpCampaigns.Name:
			_, _ = w.Write([]byte(`{"data":{"currentUser":{"dropCampaigns":[
				{"id":"live","name":"Live","status":"ACTIVE","startAt":"2026-06-01T00:00:00Z","endAt":"2099-01-01T00:00:00Z",
				 "game":{"id":"263490","displayName":"Rust"}},
				{"id":"soon","name":"Soon","status":"UPCOMING","startAt":"2098-01-01T00:00:00Z","endAt":"2099-01-01T00:00:00Z",
				 "game":{"id":"263490","displayName":"Rust"}},
				{"id":"old","name":"Old","status":"EXPIRED","startAt":"2026-01-01T00:00:00Z","endAt":"2026-02-01T00:00:00Z",
				 "game":{"id":"263490","displayName":"Rust"}}
			]}}}`))
		case OpDropCampaignDetails.Name:
			id, _ := req.Variables["dropID"].(string)
			mu.Lock()
			details = append(details, id)
			mu.Unlock()
			_, _ = w.Write(loadFixture(t, "campaign_details.json"))
		default:
			t.Fatalf("unexpected op %q", req.OperationName)
		}
	}))
	defer srv.Close()

	camps, err := newForTest(srv.URL).Catalog(context.Background(), platform.Session{AccessToken: "tok", UserID: "42"})
	require.NoError(t, err)
	byID := map[string]platform.Campaign{}
	for _, c := range camps {
		byID[c.ID] = c
	}
	require.Len(t, byID, 2, "expired left out")
	assert.Equal(t, "active", byID["live"].Status)
	up := byID["soon"]
	assert.Equal(t, "upcoming", up.Status)
	assert.Equal(t, "2098-01-01T00:00:00Z", up.StartsAt.UTC().Format(time.RFC3339))
	require.Len(t, up.Benefits, 1, "upcoming drops come from the details query")
	assert.Equal(t, 60, up.Benefits[0].RequiredMinutes)
	assert.ElementsMatch(t, []string{"live", "soon"}, details, "no details call for expired")
}
