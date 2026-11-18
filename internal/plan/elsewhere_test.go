package plan

import (
	"slices"
	"testing"
	"time"

	"github.com/grubdrops/grubdrops/internal/platform"
)

func TestClaimedElsewhere(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	after := start.Add(48 * time.Hour)
	before := start.Add(-48 * time.Hour)
	mk := func(startsAt time.Time, rewards ...string) platform.Campaign {
		c := platform.Campaign{ID: "c", StartsAt: startsAt}
		for i, r := range rewards {
			c.Benefits = append(c.Benefits, platform.DropBenefit{ID: "d" + string(rune('1'+i)), RewardID: r})
		}
		return c
	}
	aw := func(m map[string]time.Time) platform.OwnedRewards {
		return platform.OwnedRewards{AwardedAt: m, InProgress: map[string]bool{}}
	}
	inProg := aw(map[string]time.Time{"r1": after})
	inProg.InProgress["c"] = true

	tests := []struct {
		name  string
		camp  platform.Campaign
		owned platform.OwnedRewards
		want  []string
	}{
		{"all owned after start", mk(start, "r1", "r2"), aw(map[string]time.Time{"r1": after, "r2": after}), []string{"d1", "d2"}},
		{"awarded exactly at start", mk(start, "r1"), aw(map[string]time.Time{"r1": start}), []string{"d1"}},
		{"in progress", mk(start, "r1"), inProg, nil},
		{"one benefit not owned", mk(start, "r1", "r2"), aw(map[string]time.Time{"r1": after}), nil},
		{"awarded before start", mk(start, "r1"), aw(map[string]time.Time{"r1": before}), nil},
		{"zero StartsAt", mk(time.Time{}, "r1"), aw(map[string]time.Time{"r1": after}), nil},
		{"empty reward id", mk(start, ""), aw(map[string]time.Time{"": after}), nil},
		{"no benefits", mk(start), aw(nil), nil},
		{"four drops one reward", mk(start, "r1", "r1", "r1", "r1"), aw(map[string]time.Time{"r1": after}), []string{"d1", "d2", "d3", "d4"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClaimedElsewhere([]platform.Campaign{tc.camp}, tc.owned)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
