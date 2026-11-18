package plan

import "github.com/grubdrops/grubdrops/internal/platform"

// ClaimedElsewhere returns the ids of drops the account claimed outside
// GrubDrops. Twitch drops a campaign from the inventory's in-progress list
// once every drop is claimed, so those drops never show as claimed progress.
// All drops of a campaign count only when the campaign is not in progress,
// it has benefits, and every benefit's reward was awarded at or after the
// campaign start. Twitch reuses reward items across campaigns and seasons
// (#24), so owning the item alone proves nothing, and a partly claimed
// campaign stays on the per-drop inventory truth.
func ClaimedElsewhere(camps []platform.Campaign, owned platform.OwnedRewards) []string {
	var ids []string
	for _, c := range camps {
		if c.StartsAt.IsZero() || len(c.Benefits) == 0 || owned.InProgress[c.ID] {
			continue
		}
		all := true
		for _, b := range c.Benefits {
			at, ok := owned.AwardedAt[b.RewardID]
			if b.RewardID == "" || !ok || at.Before(c.StartsAt) {
				all = false
				break
			}
		}
		if !all {
			continue
		}
		for _, b := range c.Benefits {
			ids = append(ids, b.ID)
		}
	}
	return ids
}
