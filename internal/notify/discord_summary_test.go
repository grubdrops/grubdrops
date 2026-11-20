package notify

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEmbed_SummaryReport(t *testing.T) {
	r := &SummaryReport{
		Day:     time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC),
		Minutes: 1234,
		Accounts: []SummaryAccount{
			{Label: "ann_b", Platform: "kick", Minutes: 658},
			{Label: "bob", Platform: "twitch", Minutes: 576},
		},
		Drops: []SummaryDrop{
			{Game: "WARDOGS", Drop: "Walk It Off", Platform: "twitch", Account: "bob"},
			{Game: "WARDOGS", Drop: "Taxi *Service*", Platform: "twitch", Account: "bob"},
		},
	}
	e := buildEmbed(EventSummary, map[string]any{"title": "Daily summary", "msg": "x", FieldReport: r})
	assert.Equal(t, "📊 Daily summary", e["title"])
	assert.Equal(t, summaryColor, e["color"])
	assert.Equal(t, "📅 **Thursday, 19 November 2026**\n⏱ **20h 34m** watched · 🎁 **2** drops claimed", e["description"])
	fields := e["fields"].([]any)
	require.Len(t, fields, 2)
	f := func(i int) map[string]any { return fields[i].(map[string]any) }
	assert.Equal(t, "⏱ Watch time", f(0)["name"])
	assert.Equal(t, "🟢 Kick · ann\\_b — `10h 58m`\n🟣 Twitch · bob — `9h 36m`", f(0)["value"])
	assert.Equal(t, "🎮 WARDOGS 🟣 (2)", f(1)["name"])
	assert.Equal(t, "• Walk It Off\n• Taxi \\*Service\\*", f(1)["value"])
}

func TestBuildEmbed_SummaryReportLimits(t *testing.T) {
	r := &SummaryReport{Day: time.Now()}
	none := buildEmbed(EventSummary, map[string]any{FieldReport: r})
	assert.Equal(t, "None today", none["fields"].([]any)[1].(map[string]any)["value"])
	assert.Contains(t, none["description"], "🎁 **0** drops claimed")

	for g := 0; g < 40; g++ {
		for d := 0; d < 20; d++ {
			r.Drops = append(r.Drops, SummaryDrop{Game: fmt.Sprintf("Game %02d", g), Drop: strings.Repeat("x", 60), Platform: "kick"})
		}
	}
	e := buildEmbed(EventSummary, map[string]any{"title": "S", FieldReport: r})
	fields := e["fields"].([]any)
	assert.LessOrEqual(t, len(fields), discordMaxFields)
	raw, _ := json.Marshal(e)
	total := 0
	for _, x := range fields {
		m := x.(map[string]any)
		total += len(m["name"].(string)) + len(m["value"].(string))
		assert.LessOrEqual(t, len(m["value"].(string)), discordFieldValueMax)
	}
	assert.LessOrEqual(t, total, 6000, string(raw[:100]))
	last := fields[len(fields)-1].(map[string]any)
	assert.Contains(t, last["value"], "more games")
}
