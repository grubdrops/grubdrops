package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Migration 0033 moves Kick accounts left at the old off default to Drops +
// points, the mode they always ran with; Twitch accounts keep theirs.
func TestMigration_KickPointsMode(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	for _, a := range []struct{ id, plat, mode string }{{"k1", "kick", "off"}, {"k2", "kick", "points_only"}, {"t1", "twitch", "off"}} {
		_, err := db.ExecContext(ctx, `INSERT INTO accounts (id, platform, display_name, status, fingerprint_json, enabled, created_at, updated_at, points_mode)
			VALUES (?, ?, ?, 'idle', '{}', 1, 0, 0, ?)`, a.id, a.plat, a.id, a.mode)
		require.NoError(t, err)
	}
	b, err := migrationsFS.ReadFile("migrations/0033_kick_points_mode.sql")
	require.NoError(t, err)
	up := strings.SplitN(strings.SplitN(string(b), "-- +goose Down", 2)[0], "-- +goose Up", 2)[1]
	_, err = db.ExecContext(ctx, up)
	require.NoError(t, err)
	for id, want := range map[string]string{"k1": "drops_points", "k2": "points_only", "t1": "off"} {
		var got string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT points_mode FROM accounts WHERE id = ?`, id).Scan(&got))
		assert.Equal(t, want, got, id)
	}
}
