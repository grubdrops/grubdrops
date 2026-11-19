package twitch

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/grubdrops/grubdrops/internal/platform"
)

// confirmWaits bounds the inventory check after a claim: one read after
// each wait. Twitch can answer ELIGIBLE_FOR_ALL or
// DROP_INSTANCE_ALREADY_CLAIMED and still keep the drop unclaimed (GRUB-58),
// so the inventory decides. The inventory can lag a granted claim by most of
// a minute, so the waits back off to about 60s in all. A var so tests can
// shrink it.
var confirmWaits = []time.Duration{
	0, 2 * time.Second, 3 * time.Second, 5 * time.Second,
	10 * time.Second, 15 * time.Second, 25 * time.Second,
}

type claimer struct {
	c *client
}

type claimResult struct {
	ClaimDropRewards *struct {
		Status string `json:"status"`
	} `json:"claimDropRewards"`
}

// claim claims drop b and returns nil only when Twitch granted it: a success
// status that the account's inventory confirms. Any other answer wraps
// platform.ErrClaimRefused.
func (cl *claimer) claim(ctx context.Context, sess platform.Session, b platform.DropBenefit, userID int64) error {
	// Prefer the per-account instance id captured at progress time.
	// When it's missing, construct DevilXD's synthetic instance id
	// `userID#campaignID#dropID` (inventory.py generate_claim) — Twitch
	// accepts it and rejects the bare drop-template id with
	// INVALID_DROP_INSTANCE. Only fall back to the template id as a last
	// resort when we couldn't resolve the user id.
	id := b.InstanceID
	if id == "" && userID > 0 && b.CampaignID != "" && b.ID != "" {
		id = fmt.Sprintf("%d#%s#%s", userID, b.CampaignID, b.ID)
	}
	if id == "" {
		id = b.ID
	}
	var out claimResult
	err := cl.c.gql(ctx, sess.AccessToken, OpClaimDrop,
		map[string]any{"input": map[string]any{"dropInstanceID": id}}, &out)
	if err != nil {
		return fmt.Errorf("claim %s: %w", id, err)
	}
	if out.ClaimDropRewards == nil {
		return fmt.Errorf("claim %s: twitch returned no claim result (unknown or invalid drop instance): %w", id, platform.ErrClaimRefused)
	}
	status := out.ClaimDropRewards.Status
	switch status {
	case "ELIGIBLE_FOR_ALL", "DROP_INSTANCE_ALREADY_CLAIMED":
	default:
		return fmt.Errorf("claim %s: status %q: %w", id, status, platform.ErrClaimRefused)
	}
	return cl.confirm(ctx, sess, b.ID, id, status)
}

// confirm reads the inventory until it shows drop dropID claimed or gone,
// once after each of confirmWaits. Twitch drops a campaign from the inventory
// once its last drop is claimed, so gone counts as granted. Still unclaimed
// after every read means Twitch did not grant it. A failed read trusts the
// status: the inventory sweep catches a claim that did not land.
func (cl *claimer) confirm(ctx context.Context, sess platform.Session, dropID, id, status string) error {
	for _, wait := range confirmWaits {
		if wait > 0 {
			t := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				t.Stop()
				return fmt.Errorf("claim %s: confirm: %w", id, ctx.Err())
			case <-t.C:
			}
		}
		found, claimed, err := cl.inventoryClaimed(ctx, sess, dropID)
		if err != nil {
			slog.Warn("twitch claim unconfirmed: inventory read failed; trusting the claim status",
				"kind", "claim", "account", sess.AccountID, "drop", dropID, "status", status, "err", err)
			return nil
		}
		if !found || claimed {
			return nil
		}
	}
	return fmt.Errorf("claim %s: twitch answered %s but the inventory still shows the drop unclaimed: %w",
		id, status, platform.ErrClaimRefused)
}

// inventoryClaimed reports whether the account's inventory lists drop dropID
// and whether it is claimed there.
func (cl *claimer) inventoryClaimed(ctx context.Context, sess platform.Session, dropID string) (found, claimed bool, err error) {
	var inv inventoryData
	if err := cl.c.gql(ctx, sess.AccessToken, OpInventory, nil, &inv); err != nil {
		return false, false, err
	}
	for _, camp := range inv.CurrentUser.Inventory.DropCampaignsInProgress {
		for _, td := range camp.TimeBasedDrops {
			if td.ID == dropID {
				return true, td.Self.IsClaimed, nil
			}
		}
	}
	return false, false, nil
}

// claimRepeat settles one day of a repeatable watch group. Twitch grants the
// reward by itself, so a day without an instance id has nothing to claim.
// With one (the group reported CLAIMABLE), the claim mutation goes out and
// a success status counts: there is no inventory row to confirm against.
func (cl *claimer) claimRepeat(ctx context.Context, sess platform.Session, b platform.DropBenefit) error {
	if b.InstanceID == "" {
		return nil
	}
	var out claimResult
	err := cl.c.gql(ctx, sess.AccessToken, OpClaimDrop,
		map[string]any{"input": map[string]any{"dropInstanceID": b.InstanceID}}, &out)
	if err != nil {
		return fmt.Errorf("claim %s: %w", b.InstanceID, err)
	}
	if out.ClaimDropRewards == nil {
		return fmt.Errorf("claim %s: twitch returned no claim result: %w", b.InstanceID, platform.ErrClaimRefused)
	}
	switch status := out.ClaimDropRewards.Status; status {
	case "ELIGIBLE_FOR_ALL", "DROP_INSTANCE_ALREADY_CLAIMED":
		return nil
	default:
		return fmt.Errorf("claim %s: status %q: %w", b.InstanceID, status, platform.ErrClaimRefused)
	}
}
