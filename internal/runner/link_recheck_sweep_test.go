package runner

import (
	"context"
	"testing"

	"github.com/aalejandrofer/grubdrops/internal/platform"
	"github.com/aalejandrofer/grubdrops/internal/platform/platformtest"
)

// A Twitch drop whose campaign ended has left the plan: Check now sweeps the
// inventory, so a drop of a now-linked campaign is claimed and one still
// unlinked is counted, not claimed.
func TestRunner_RecheckLinksSweepsReadyDrops(t *testing.T) {
	noSweepGap(t)
	f := platformtest.NewFake()
	linked := readyDrop("ok")
	unlinked := readyDrop("wait")
	unlinked.Campaign.ID, unlinked.Benefit.CampaignID = "pub", "pub"
	unlinked.Campaign.AccountLinkURL = "https://publisher.example/link"
	f.Ready = []platform.ReadyDrop{linked, unlinked}
	cl := &fakeClaims{}
	r := New(Config{AccountID: "acc", Platform: "twitch", Backend: f, Games: rustGames, Claims: cl})

	got, err := r.RecheckLinks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != (LinkRecheck{Claimed: 1, StillNeedsLink: 1}) {
		t.Fatalf("got=%+v", got)
	}
	if f.CallCount("Claim:ok") != 1 || f.CallCount("Claim:wait") != 0 {
		t.Fatalf("calls=%v", f.Calls)
	}
	if rec := cl.recorded(); len(rec) != 1 || rec[0] != "acc:ok:" {
		t.Fatalf("recorded=%v", rec)
	}
	if !r.forceScan.Load() {
		t.Fatal("recheck must rescan")
	}
}
