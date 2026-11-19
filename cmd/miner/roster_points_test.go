package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grubdrops/grubdrops/internal/platform"
	"github.com/grubdrops/grubdrops/internal/points"
	"github.com/grubdrops/grubdrops/internal/store"
)

// balancerBackend is a backend that reads channel points balances.
type balancerBackend struct {
	platform.Backend
}

func (balancerBackend) ChannelPointsBalance(context.Context, platform.Session, string) (int, error) {
	return 20, nil
}

// A Kick account, whose points mode is always effectively on, keeps the
// balances of its Points watch channels fresh like a Twitch one.
func TestRoster_PointsTargetsIncludeKick(t *testing.T) {
	e := newRosterEnv(t)
	e.withForce()
	e.account(t, "k1", 1, true) // Kick, stored points mode off
	e.account(t, "k2", 1, true) // Kick, no balance reader loaded
	e.setForce(t, []store.ForceChannel{
		{Platform: "kick", Channel: "somestreamer", Accounts: []string{"k1", "k2"}},
	})
	e.ro.loaded.set("k1", loadedSession{backend: balancerBackend{e.fake}})
	e.ro.loaded.set("k2", loadedSession{backend: e.fake})

	got, err := e.ro.pointsTargets(e.ctx)
	require.NoError(t, err)
	assert.Equal(t, []points.Target{{AccountID: "k1", Channel: "somestreamer"}}, got)

	n, err := e.ro.fetchPointsBalance(e.ctx, "k1", "somestreamer")
	require.NoError(t, err)
	assert.Equal(t, 20, n)
}
