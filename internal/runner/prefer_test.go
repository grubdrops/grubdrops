package runner

import (
	"slices"
	"testing"

	"github.com/grubdrops/grubdrops/internal/platform"
	"github.com/grubdrops/grubdrops/internal/platform/platformtest"
)

func channelsOf(ss []platform.Stream) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Channel
	}
	return out
}

// Force list channels move to the front in list order, case-insensitively;
// the rest keep their order.
func TestPreferForce_Order(t *testing.T) {
	in := []platform.Stream{{Channel: "big"}, {Channel: "Fav2"}, {Channel: "mid"}, {Channel: "fav1"}}
	got := channelsOf(preferForce(in, []string{"FAV1", "fav2", "offline"}))
	if want := []string{"fav1", "Fav2", "big", "mid"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := channelsOf(preferForce(in, nil)); !slices.Equal(got, channelsOf(in)) {
		t.Fatalf("no force list reordered: %v", got)
	}
	if in[0].Channel != "big" {
		t.Fatal("input slice modified")
	}
}

// A drop watch picks a Points watch channel the campaign counts on over a
// bigger channel.
func TestDropWatch_PrefersForceChannel(t *testing.T) {
	f := platformtest.NewFake()
	f.Campaigns = []platform.Campaign{camp("c1", "Rust", "d1")}
	f.Campaigns[0].Benefits[0].RequiredMinutes = 100000
	f.Progress["d1"] = &platform.Progress{BenefitID: "d1"}
	f.Streams["c1"] = []platform.Stream{{Channel: "big", ViewerCount: 5000}, {Channel: "fav", ViewerCount: 12}}
	r := newRunner(f, forceCfg(f, "fav"))
	steps(t, r, 2) // pick, start
	if s := r.Snapshot(); s.State != StateWatching || s.Channel != "fav" {
		t.Fatalf("want a drop watch on fav, got %+v", s)
	}
}

// Without the channel on the list, the biggest channel wins as before.
func TestDropWatch_BiggestWithoutForceChannel(t *testing.T) {
	f := platformtest.NewFake()
	f.Campaigns = []platform.Campaign{camp("c1", "Rust", "d1")}
	f.Campaigns[0].Benefits[0].RequiredMinutes = 100000
	f.Progress["d1"] = &platform.Progress{BenefitID: "d1"}
	f.Streams["c1"] = []platform.Stream{{Channel: "fav", ViewerCount: 12}, {Channel: "big", ViewerCount: 5000}}
	r := newRunner(f, forceCfg(f, "other"))
	steps(t, r, 2)
	if s := r.Snapshot(); s.State != StateWatching || s.Channel != "big" {
		t.Fatalf("want a drop watch on big, got %+v", s)
	}
}

// favLive scripts a live lookup for login streaming game.
func favLive(f *platformtest.Fake, login, game string) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.Lookups[login] = platformtest.Lookup{
		Stream: platform.Stream{Channel: login, ChannelID: "id-" + login, ViewerCount: 3, Game: game, DropsEnabled: true},
		Live:   true,
	}
}

func openCampaign(f *platformtest.Fake) {
	f.Campaigns = []platform.Campaign{camp("c1", "Rust", "d1")}
	f.Campaigns[0].Benefits[0].RequiredMinutes = 100000
	f.Progress["d1"] = &platform.Progress{BenefitID: "d1"}
	f.Streams["c1"] = []platform.Stream{{Channel: "big", ViewerCount: 5000}}
}

// A small Points watch channel live on an open campaign's game, missing from
// the backend's busiest-first list, is added and watched first.
func TestDropWatch_AddsForceChannelOnOpenCampaign(t *testing.T) {
	f := platformtest.NewFake()
	openCampaign(f)
	favLive(f, "fav", "rust")
	r := newRunner(f, forceCfg(f, "fav"))
	steps(t, r, 2)
	if s := r.Snapshot(); s.State != StateWatching || s.Channel != "fav" {
		t.Fatalf("want a drop watch on fav, got %+v", s)
	}
}

// A Points watch channel on another game is not added.
func TestDropWatch_SkipsForceChannelOnOtherGame(t *testing.T) {
	f := platformtest.NewFake()
	openCampaign(f)
	favLive(f, "fav", "Minecraft")
	r := newRunner(f, forceCfg(f, "fav"))
	steps(t, r, 2)
	if s := r.Snapshot(); s.State != StateWatching || s.Channel != "big" {
		t.Fatalf("want a drop watch on big, got %+v", s)
	}
}

// A restricted campaign keeps its own channels only.
func TestDropWatch_RestrictedCampaignKeepsItsChannels(t *testing.T) {
	f := platformtest.NewFake()
	openCampaign(f)
	f.Campaigns[0].AllowedChannels = []string{"big"}
	f.Campaigns[0].AllowedChannelCount = 1
	favLive(f, "fav", "Rust")
	r := newRunner(f, forceCfg(f, "fav"))
	steps(t, r, 2)
	if s := r.Snapshot(); s.State != StateWatching || s.Channel != "big" {
		t.Fatalf("want a drop watch on big, got %+v", s)
	}
}

// With no eligible stream listed, a live Points watch channel on the game
// still lets the drop be mined.
func TestDropWatch_ForceChannelFillsEmptyList(t *testing.T) {
	f := platformtest.NewFake()
	openCampaign(f)
	f.Streams["c1"] = nil
	favLive(f, "fav", "Rust")
	r := newRunner(f, forceCfg(f, "fav"))
	steps(t, r, 2)
	if s := r.Snapshot(); s.State != StateWatching || s.Channel != "fav" || s.Current == nil {
		t.Fatalf("want a drop watch on fav, got %+v", s)
	}
}
