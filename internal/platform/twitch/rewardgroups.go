package twitch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/grubdrops/grubdrops/internal/gameslug"
	"github.com/grubdrops/grubdrops/internal/platform"
)

// Twitch "reward groups": a campaign that hangs off a category and a channel
// instead of the drops dashboard (GTAVI Daily Chest, #90). Only the
// repeatable, auto-granted watch group is mined. Each day of it becomes its
// own benefit (<groupID>:day01..), chained to the day before by
// Preconditions, so the claim log and the planner treat a day like any other
// drop. Raw queries: Twitch publishes no persisted hash for these. Neither
// goes out with a Client-Integrity header.
const (
	rewardGroupsQuery = `query ChannelDropsCampaigns($channelID:ID!){ channelDropCampaigns(channelID:$channelID){
  id name brandName detailsURL startAt endAt imageURL game{id name}
  rewardGroups{ id name
    progressCriteria{ requirementType requirements{minutesWatched subs turboSubs} isRepeatable isAutoProgressable
      repeatableConfig{repeatableTimes resetIntervalSeconds isLastChanceExtensionEnabled shouldCompleteAllRepeatsToEarn}
      channels{id} categories{id displayName} }
    rewards{id name thumbnailURL distributionType} } } }`

	rewardProgressQuery = `query DropChannelCampaignsProgress($channelID:ID!){ channelDropCampaignsProgress(channelID:$channelID){ id name
  rewardGroups{ id name
    progressCriteria{ requirementType requirements{minutesWatched subs turboSubs} isRepeatable isAutoProgressable
      repeatableConfig{repeatableTimes resetIntervalSeconds}
      channels{id} categories{id displayName} }
    rewards{id name thumbnailURL distributionType}
    self{ status currentMinutesWatched grantCount nextEligibleAt currentWindow{startedAt expiresAt} } } } }`

	// rgTTL caches a game's reward group campaigns, empty answers included.
	rgTTL = 30 * time.Minute
	// rgFailTTL is how long a game whose reward groups could not be read is
	// left alone, so a broken query is not retried on every scan.
	rgFailTTL = 10 * time.Minute
	// rgDirTTL is how long a directory read of the TV walk is reused.
	rgDirTTL = time.Minute
	// rgProgressTTL caches the progress read.
	rgProgressTTL = 30 * time.Second
	// rgProgressOff is how long the progress query rests once Twitch
	// integrity-blocks it. The runner falls back to local tracking.
	rgProgressOff = 6 * time.Hour
)

type rgCampaignsData struct {
	ChannelDropCampaigns []rgCampaign `json:"channelDropCampaigns"`
}

type rgProgressData struct {
	ChannelDropCampaignsProgress []rgCampaign `json:"channelDropCampaignsProgress"`
}

type rgCampaign struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	StartAt  string `json:"startAt"`
	EndAt    string `json:"endAt"`
	ImageURL string `json:"imageURL"`
	Game     *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"game"`
	RewardGroups []rgGroup `json:"rewardGroups"`
}

type rgGroup struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	ProgressCriteria rgCriteria `json:"progressCriteria"`
	Rewards          []struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		ThumbnailURL string `json:"thumbnailURL"`
	} `json:"rewards"`
	Self *rgSelf `json:"self"`
}

type rgCriteria struct {
	RequirementType string `json:"requirementType"`
	Requirements    struct {
		MinutesWatched int `json:"minutesWatched"`
		Subs           int `json:"subs"`
		TurboSubs      int `json:"turboSubs"`
	} `json:"requirements"`
	IsRepeatable       bool `json:"isRepeatable"`
	IsAutoProgressable bool `json:"isAutoProgressable"`
	RepeatableConfig   *struct {
		RepeatableTimes      int   `json:"repeatableTimes"`
		ResetIntervalSeconds int64 `json:"resetIntervalSeconds"`
	} `json:"repeatableConfig"`
	Channels []struct {
		ID string `json:"id"`
	} `json:"channels"`
	Categories []struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
	} `json:"categories"`
}

type rgSelf struct {
	Status                string `json:"status"`
	CurrentMinutesWatched int    `json:"currentMinutesWatched"`
	GrantCount            int    `json:"grantCount"`
	NextEligibleAt        string `json:"nextEligibleAt"`
	CurrentWindow         *struct {
		StartedAt string `json:"startedAt"`
		ExpiresAt string `json:"expiresAt"`
	} `json:"currentWindow"`
}

// repeatBenefitID is the synthetic benefit id of one day of a group.
func repeatBenefitID(groupID string, day int) string {
	return fmt.Sprintf("%s:day%02d", groupID, day)
}

// rgDays reports how many days a group runs and whether it is a group to
// mine: a watch-time group that Twitch grants by itself, needs no
// subscription and accepts any channel of its category.
func rgDays(g rgGroup) (days int, ok bool) {
	pc := g.ProgressCriteria
	if pc.RequirementType != "WATCH" || pc.Requirements.MinutesWatched <= 0 ||
		pc.Requirements.Subs != 0 || pc.Requirements.TurboSubs != 0 ||
		!pc.IsAutoProgressable || len(pc.Channels) > 0 {
		return 0, false
	}
	days = 1
	if pc.IsRepeatable && pc.RepeatableConfig != nil && pc.RepeatableConfig.RepeatableTimes > 0 {
		days = pc.RepeatableConfig.RepeatableTimes
	}
	return days, true
}

// mapRewardCampaign turns c into a campaign of its mineable watch groups, one
// benefit per group day. ok is false when no group qualifies. fallbackGame
// names the game the campaign was queried for, used when Twitch names none.
func mapRewardCampaign(c rgCampaign, fallbackGame string, now time.Time) (platform.Campaign, bool) {
	out := platform.Campaign{
		ID:            c.ID,
		Platform:      "twitch",
		Name:          c.Name,
		StartsAt:      parseISO(c.StartAt),
		EndsAt:        parseISO(c.EndAt),
		Kind:          platform.KindRepeat,
		AccountLinked: true,
	}
	out.Status = windowStatus(out.StartsAt, out.EndsAt, now)
	game, gameID := fallbackGame, ""
	if c.Game != nil {
		if c.Game.Name != "" {
			game = c.Game.Name
		}
		gameID = c.Game.ID
	}
	for _, g := range c.RewardGroups {
		days, ok := rgDays(g)
		if !ok {
			continue
		}
		pc := g.ProgressCriteria
		if len(pc.Categories) > 0 && len(out.Benefits) == 0 {
			game, gameID = pc.Categories[0].DisplayName, pc.Categories[0].ID
		}
		rep := platform.Repeat{GroupID: g.ID, Days: days}
		if pc.RepeatableConfig != nil {
			rep.Reset = time.Duration(pc.RepeatableConfig.ResetIntervalSeconds) * time.Second
		}
		for _, cat := range pc.Categories {
			rep.CategoryIDs = append(rep.CategoryIDs, cat.ID)
		}
		name, img := g.Name, c.ImageURL
		if len(g.Rewards) > 0 {
			name, img = g.Rewards[0].Name, g.Rewards[0].ThumbnailURL
		}
		for day := 1; day <= days; day++ {
			b := platform.DropBenefit{
				ID:              repeatBenefitID(g.ID, day),
				CampaignID:      c.ID,
				Name:            name,
				RequiredMinutes: pc.Requirements.MinutesWatched,
				ImageURL:        img,
			}
			if day > 1 {
				b.Preconditions = []string{repeatBenefitID(g.ID, day-1)}
			}
			r := rep
			r.Day = day
			b.Repeat = &r
			out.Benefits = append(out.Benefits, b)
		}
	}
	if len(out.Benefits) == 0 {
		return platform.Campaign{}, false
	}
	out.Game, out.GameID = game, gameID
	if gameID != "" {
		out.BoxArtURL = BoxArtURL("", gameID)
	}
	return out, true
}

// progressRows turns Twitch's own view of c's mineable groups into one row
// per finished day, plus the row of the day in progress. A group Twitch
// reports nothing on yields no rows.
func progressRows(c rgCampaign, userID string, now time.Time) []platform.Progress {
	var out []platform.Progress
	for _, g := range c.RewardGroups {
		days, ok := rgDays(g)
		if !ok || g.Self == nil {
			continue
		}
		req := g.ProgressCriteria.Requirements.MinutesWatched
		done := min(max(g.Self.GrantCount, 0), days)
		for day := 1; day <= done; day++ {
			out = append(out, platform.Progress{BenefitID: repeatBenefitID(g.ID, day), MinutesWatched: req, Claimed: true})
		}
		if done >= days {
			continue
		}
		p := platform.Progress{
			BenefitID:      repeatBenefitID(g.ID, done+1),
			MinutesWatched: g.Self.CurrentMinutesWatched,
			NextEligibleAt: parseISO(g.Self.NextEligibleAt),
		}
		// Minutes left over from the day before a grant are stale: they count
		// only inside a window that opened at or after the grant.
		stale := false
		if done > 0 {
			w := g.Self.CurrentWindow
			stale = w == nil || parseISO(w.StartedAt).IsZero() ||
				(!p.NextEligibleAt.IsZero() && parseISO(w.StartedAt).Before(p.NextEligibleAt))
		}
		expired := g.Self.CurrentWindow != nil && !parseISO(g.Self.CurrentWindow.ExpiresAt).IsZero() &&
			now.After(parseISO(g.Self.CurrentWindow.ExpiresAt))
		if now.Before(p.NextEligibleAt) || expired || stale {
			p.MinutesWatched = 0
		}
		if g.Self.Status == "CLAIMABLE" {
			p.MinutesWatched = max(p.MinutesWatched, req)
			if userID != "" {
				p.InstanceID = userID + "#" + c.ID + "#" + g.ID
			}
		}
		out = append(out, p)
	}
	return out
}

// rgEntry is one game's cached reward group campaigns.
type rgEntry struct {
	camps []platform.Campaign
	at    time.Time
}

// rewardGroupCampaigns returns the reward group campaigns of the account's
// games, skipping ids in have. It never fails: a game it cannot read yields
// nothing this pass.
func (d *discovery) rewardGroupCampaigns(ctx context.Context, sess platform.Session, ch *channels, have map[string]bool) []platform.Campaign {
	var out []platform.Campaign
	seen := make(map[string]bool, len(sess.Games))
	for _, game := range sess.Games {
		if gameslug.Slug(game) == "" {
			continue
		}
		slug := ch.dirSlug(ctx, sess, "", game)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		d.mu.Lock()
		e, hit := d.rgByGame[slug]
		d.mu.Unlock()
		if !hit || time.Since(e.at) >= rgTTL {
			camps, ok := d.fetchRewardGroups(ctx, sess, ch, game, slug)
			if ctx.Err() != nil {
				// A canceled pass says nothing about the game; cache nothing.
				return out
			}
			e = rgEntry{camps: camps, at: time.Now()}
			if !ok {
				// Cached as empty, expiring after rgFailTTL.
				e.at = e.at.Add(rgFailTTL - rgTTL)
			}
			d.mu.Lock()
			if d.rgByGame == nil {
				d.rgByGame = map[string]rgEntry{}
			}
			d.rgByGame[slug] = e
			d.mu.Unlock()
		}
		for _, c := range e.camps {
			if have[c.ID] || (sess.GameFilter != nil && !sess.GameFilter(c.Game)) {
				continue
			}
			have[c.ID] = true
			out = append(out, c)
		}
	}
	return out
}

// fetchRewardGroups reads the reward group campaigns of one game through its
// top live channel. ok is false when Twitch could not be asked (not cached).
func (d *discovery) fetchRewardGroups(ctx context.Context, sess platform.Session, ch *channels, game, slug string) ([]platform.Campaign, bool) {
	streams, err := d.recentDirectory(slug), error(nil)
	if len(streams) == 0 {
		streams, err = ch.listDirectory(ctx, sess, slug, true)
	}
	if err == nil && len(streams) == 0 {
		streams, err = ch.listDirectory(ctx, sess, slug, false)
	}
	if err != nil {
		slog.Debug("twitch reward groups: directory failed", "game", game, "err", err)
		return nil, false
	}
	if len(streams) == 0 || streams[0].ChannelID == "" {
		return nil, true
	}
	channelID := streams[0].ChannelID
	var resp rgCampaignsData
	if err := d.c.gqlQuery(ctx, sess.AccessToken, "ChannelDropsCampaigns", rewardGroupsQuery,
		map[string]any{"channelID": channelID}, &resp); err != nil {
		slog.Debug("twitch reward groups: campaigns query failed", "game", game, "err", err)
		return nil, false
	}
	now := time.Now()
	var camps []platform.Campaign
	for _, rc := range resp.ChannelDropCampaigns {
		c, ok := mapRewardCampaign(rc, game, now)
		if !ok {
			continue
		}
		camps = append(camps, c)
		d.mu.Lock()
		if d.rgChannel == nil {
			d.rgChannel = map[string]string{}
		}
		d.rgChannel[c.ID] = channelID
		d.mu.Unlock()
	}
	return camps, true
}

// noteDirectory remembers a drops-enabled directory read of slug, so reward
// group discovery on the same pass can reuse it.
func (d *discovery) noteDirectory(slug string, streams []platform.Stream) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rgDir == nil {
		d.rgDir = map[string]rgDirEntry{}
	}
	d.rgDir[slug] = rgDirEntry{streams: streams, at: time.Now()}
}

// recentDirectory returns the directory read noteDirectory kept for slug, or
// nil when there is none within rgDirTTL.
func (d *discovery) recentDirectory(slug string) []platform.Stream {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e, ok := d.rgDir[slug]; ok && time.Since(e.at) < rgDirTTL {
		return e.streams
	}
	return nil
}

type rgDirEntry struct {
	streams []platform.Stream
	at      time.Time
}

// activeChannelsLocked returns the channels the live reward group campaigns
// were read through: unexpired cache entries, campaigns not yet ended.
// Caller holds d.mu.
func (d *discovery) activeChannelsLocked(now time.Time) map[string]bool {
	out := map[string]bool{}
	for _, e := range d.rgByGame {
		if now.Sub(e.at) >= rgTTL {
			continue
		}
		for _, c := range e.camps {
			if !c.EndsAt.IsZero() && !c.EndsAt.After(now) {
				continue
			}
			if ch := d.rgChannel[c.ID]; ch != "" {
				out[ch] = true
			}
		}
	}
	return out
}

// hasRewardGroups reports whether a live reward group campaign is cached.
func (d *discovery) hasRewardGroups() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.activeChannelsLocked(time.Now())) > 0
}

// rewardGroupBenefits returns the cached benefits of reward group campaign id.
func (d *discovery) rewardGroupBenefits(id string) ([]platform.DropBenefit, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, e := range d.rgByGame {
		for _, c := range e.camps {
			if c.ID == id {
				return c.Benefits, true
			}
		}
	}
	return nil, false
}

// rewardGroupProgress returns Twitch's progress rows for the cached reward
// group campaigns, read once per channel and cached briefly. It never fails:
// the progress query may be integrity-gated, and an error returned here would
// send the runner to re-login. A block stops the query for rgProgressOff and
// the runner tracks progress itself meanwhile.
func (d *discovery) rewardGroupProgress(ctx context.Context, sess platform.Session, userID string) []platform.Progress {
	now := time.Now()
	d.mu.Lock()
	if now.Before(d.rgProgOffUntil) || now.Sub(d.rgProgAt) < rgProgressTTL {
		rows := d.rgProg
		d.mu.Unlock()
		return rows
	}
	channels := d.activeChannelsLocked(now)
	d.mu.Unlock()

	var rows []platform.Progress
	blocked, failed := false, false
	for channelID := range channels {
		var resp rgProgressData
		err := d.c.gqlQuery(ctx, sess.AccessToken, "DropChannelCampaignsProgress", rewardProgressQuery,
			map[string]any{"channelID": channelID}, &resp)
		if errors.Is(err, ErrIntegrityBlocked) {
			blocked = true
			slog.Info("twitch reward group progress is integrity-blocked; tracking it locally",
				"kind", "discovery", "account", sess.AccountID)
			break
		}
		if err != nil {
			slog.Debug("twitch reward group progress failed", "channel", channelID, "err", err)
			failed = true
			continue
		}
		for _, c := range resp.ChannelDropCampaignsProgress {
			for _, g := range c.RewardGroups {
				if g.Self != nil {
					raw, _ := json.Marshal(g.Self)
					slog.Debug("twitch reward group self", "campaign", c.ID, "group", g.ID, "self", string(raw))
				}
			}
			rows = append(rows, progressRows(c, userID, now)...)
		}
	}
	d.mu.Lock()
	// Only a pass that read every channel replaces the rows: a partial one
	// would drop days Twitch reported, and the last good rows stay.
	if !blocked && !failed {
		d.rgProg = rows
	}
	d.rgProgAt = now
	if blocked {
		d.rgProgOffUntil = now.Add(rgProgressOff)
	}
	out := d.rgProg
	d.mu.Unlock()
	return out
}
