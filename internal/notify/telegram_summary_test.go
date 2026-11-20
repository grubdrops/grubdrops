package notify

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTelegram_SummaryReport(t *testing.T) {
	srv, calls := tgServer(t, okHandler)
	bot := newTestBot(srv.URL, nil)
	r := &SummaryReport{
		Day:     time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC),
		Minutes: 1234,
		Accounts: []SummaryAccount{
			{Label: "Ann<a>", Platform: "kick", Minutes: 658},
			{Label: "bob", Platform: "twitch", Minutes: 576},
		},
		Drops: []SummaryDrop{
			{Game: "WARDOGS", Drop: "Walk It Off", Platform: "twitch", Account: "bob"},
			{Game: "GTA VI", Drop: "Space & Jam", Platform: "twitch", Account: "bob"},
			{Game: "WARDOGS", Drop: "Taxi Service", Platform: "twitch", Account: "bob"},
		},
	}
	require.NoError(t, bot.Notify(context.Background(), EventSummary, map[string]any{
		"title": "Daily summary", "msg": "ignored", FieldReport: r,
	}))
	got := calls()
	require.Len(t, got, 1)
	assert.Equal(t, strings.Join([]string{
		"📊 <b>Daily summary</b>",
		"📅 <i>2026-11-19</i>",
		"",
		"⏱ <b>Watch time:</b> <code>20h 34m</code>",
		"🟢 Kick · Ann&lt;a&gt; — <code>10h 58m</code>",
		"🟣 Twitch · bob — <code>9h 36m</code>",
		"",
		"🎁 <b>Drops claimed:</b> <code>3</code>",
		"",
		"🎮 <b>WARDOGS</b> 🟣 <i>(2)</i>",
		"   • Walk It Off",
		"   • Taxi Service",
		"",
		"🎮 <b>GTA VI</b> 🟣 <i>(1)</i>",
		"   • Space &amp; Jam",
		"",
		"<i>GrubDrops</i>",
	}, "\n"), got[0].body["text"])
	assert.Equal(t, "HTML", got[0].body["parse_mode"])
}

func TestTelegramSummary_NoDropsAndManyAccounts(t *testing.T) {
	r := &SummaryReport{Day: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Minutes: 45}
	assert.Contains(t, telegramSummary("S", r), "🎁 <b>Drops claimed:</b> none")
	assert.Contains(t, telegramSummary("S", r), "<code>45m</code>")

	r.Drops = []SummaryDrop{
		{Game: "G", Drop: "A", Platform: "kick", Account: "one"},
		{Game: "G", Drop: "B", Platform: "kick", Account: "two"},
	}
	out := telegramSummary("S", r)
	assert.Contains(t, out, "   • A <i>· one</i>")
	assert.Contains(t, out, "🎮 <b>G</b> 🟢 <i>(2)</i>")
}

func TestTelegramSummary_CapsLongLists(t *testing.T) {
	r := &SummaryReport{Day: time.Now()}
	for i := 0; i < maxSummaryDropsPerGame+3; i++ {
		r.Drops = append(r.Drops, SummaryDrop{Game: "G", Drop: "D", Platform: "twitch"})
	}
	assert.Contains(t, telegramSummary("S", r), "   <i>+3 more</i>")

	r.Drops = nil
	for i := 0; i < 400; i++ {
		r.Drops = append(r.Drops, SummaryDrop{Game: strings.Repeat("g", 10) + string(rune('a'+i%26)) + string(rune('a'+i/26)), Drop: "D"})
	}
	out := telegramSummary("S", r)
	assert.LessOrEqual(t, len([]rune(out)), telegramMessageLimit)
	assert.True(t, strings.HasSuffix(out, "<i>… cut</i>"))
}
