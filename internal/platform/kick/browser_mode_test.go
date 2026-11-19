package kick

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Auto-created per-account sidecars are enough for the browser and auto
// watch modes: no always-on sidecar client is needed.
func TestBackend_BrowserWatchWithAutoCreatedSidecars(t *testing.T) {
	b := New(nil, newFakeCtl(), "grubdrops-browser-{slug}", 9090, time.Minute, WithSidecarAutoCreate("img", ""))
	require.True(t, b.CanBrowse())
	b.EnableBrowserWatch()
	assert.True(t, b.browserWatch, "browser watch on with auto-created sidecars")
	b.EnableAutoWatch()
	assert.True(t, b.autoWatch)

	// No controller, no image, no client: the WebSocket path stays.
	for _, nb := range []*Backend{
		New(nil, nil, "grubdrops-browser-{slug}", 9090, time.Minute, WithSidecarAutoCreate("img", "")),
		New(nil, newFakeCtl(), "grubdrops-browser-{slug}", 9090, time.Minute),
	} {
		assert.False(t, nb.CanBrowse())
		nb.EnableBrowserWatch()
		nb.EnableAutoWatch()
		assert.False(t, nb.browserWatch)
		assert.False(t, nb.autoWatch)
	}
}
