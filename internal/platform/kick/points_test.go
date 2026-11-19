package kick

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grubdrops/grubdrops/internal/platform"
)

func TestKickBackend_ChannelPointsBalance(t *testing.T) {
	f := &fakeDoer{resp: map[string]fakeResp{
		"https://kick.com/api/v2/channels/somestreamer/points": {200, `{"data":{"points":20},"message":"OK"}`},
		"https://kick.com/api/v2/channels/nopoints/points":     {200, `{"data":{},"message":"OK"}`},
		"https://kick.com/api/v2/channels/broken/points":       {500, `{}`},
	}}
	b := withFake(f)
	ctx := context.Background()

	n, err := b.ChannelPointsBalance(ctx, sess("acc1"), "somestreamer")
	require.NoError(t, err)
	assert.Equal(t, 20, n)
	require.Len(t, f.calls, 1)
	assert.Equal(t, "GET", f.calls[0].method)

	_, err = b.ChannelPointsBalance(ctx, sess("acc1"), "nopoints")
	assert.ErrorIs(t, err, platform.ErrNoPointsBalance)
	_, err = b.ChannelPointsBalance(ctx, sess("acc1"), "missing") // fake answers 404
	assert.ErrorIs(t, err, platform.ErrNoPointsBalance)
	_, err = b.ChannelPointsBalance(ctx, sess("acc1"), "broken")
	require.Error(t, err)
	assert.NotErrorIs(t, err, platform.ErrNoPointsBalance)
	_, err = b.ChannelPointsBalance(ctx, sess("acc1"), "")
	require.Error(t, err)
}
