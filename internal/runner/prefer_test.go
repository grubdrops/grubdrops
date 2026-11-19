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
