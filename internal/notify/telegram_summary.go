package notify

import (
	"fmt"
	"html"
	"strings"

	"github.com/grubdrops/grubdrops/internal/i18n"
)

// telegramMessageLimit is the Bot API text length limit for sendMessage.
const telegramMessageLimit = 4096

// maxSummaryDropsPerGame caps the drops listed under one game; the rest
// are counted in a "+N more" line.
const maxSummaryDropsPerGame = 12

// telegramSummary renders a daily summary report as Bot API HTML: the day,
// watch time per account, then the drops claimed grouped by game.
func telegramSummary(title string, r *SummaryReport) string {
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString("📊 <b>" + esc(title) + "</b>\n")
	b.WriteString("📅 <i>" + esc(r.Day.Format("2006-01-02")) + "</i>\n\n")

	b.WriteString("⏱ <b>" + esc(i18n.T(r.Lang, "summary.watch_time")) + ":</b> <code>" + esc(formatMinutes(r.Minutes)) + "</code>\n")
	for _, a := range r.Accounts {
		name := a.Label
		if p := platformLabel(a.Platform); p != "" {
			name = p + " · " + name
		}
		b.WriteString(platformDot(a.Platform) + " " + esc(name) + " — <code>" + esc(formatMinutes(a.Minutes)) + "</code>\n")
	}
	b.WriteString("\n")

	if len(r.Drops) == 0 {
		b.WriteString("🎁 <b>" + esc(i18n.T(r.Lang, "summary.drops_claimed")) + ":</b> " + esc(i18n.T(r.Lang, "summary.none")) + "\n")
	} else {
		b.WriteString(fmt.Sprintf("🎁 <b>%s:</b> <code>%d</code>\n", esc(i18n.T(r.Lang, "summary.drops_claimed")), len(r.Drops)))
		manyAccounts := len(distinct(r.Drops, func(d SummaryDrop) string { return d.Account })) > 1
		for _, g := range groupDrops(r.Drops) {
			name := g.game
			if name == "" {
				name = i18n.T(r.Lang, "summary.other_game")
			}
			b.WriteString(fmt.Sprintf("\n🎮 <b>%s</b> %s <i>(%d)</i>\n", esc(name), platformDot(g.platform), len(g.drops)))
			for i, d := range g.drops {
				if i == maxSummaryDropsPerGame {
					b.WriteString("   <i>" + esc(fmt.Sprintf(i18n.T(r.Lang, "summary.more_drops"), len(g.drops)-i)) + "</i>\n")
					break
				}
				item := d.Drop
				if item == "" {
					item = i18n.T(r.Lang, "summary.drop")
				}
				line := "   • " + esc(item)
				if manyAccounts && d.Account != "" {
					line += " <i>· " + esc(d.Account) + "</i>"
				}
				b.WriteString(line + "\n")
			}
		}
	}
	b.WriteString("\n<i>GrubDrops</i>")

	out := b.String()
	if len([]rune(out)) > telegramMessageLimit {
		// Lists this long are rare; cut on a line and say so.
		runes := []rune(out)[:telegramMessageLimit-20]
		cut := string(runes)
		if i := strings.LastIndex(cut, "\n"); i > 0 {
			cut = cut[:i]
		}
		out = cut + "\n<i>… " + esc(i18n.T(r.Lang, "summary.cut")) + "</i>"
	}
	return out
}

type dropGroup struct {
	game, platform string
	drops          []SummaryDrop
}

// groupDrops groups drops by game and platform, in order of first claim.
func groupDrops(drops []SummaryDrop) []dropGroup {
	var out []dropGroup
	idx := map[string]int{}
	for _, d := range drops {
		k := d.Platform + "\x00" + d.Game
		i, ok := idx[k]
		if !ok {
			i = len(out)
			idx[k] = i
			out = append(out, dropGroup{game: d.Game, platform: d.Platform})
		}
		out[i].drops = append(out[i].drops, d)
	}
	return out
}

func distinct(drops []SummaryDrop, key func(SummaryDrop) string) map[string]bool {
	m := map[string]bool{}
	for _, d := range drops {
		m[key(d)] = true
	}
	return m
}

// platformDot is a coloured mark for a platform: Twitch purple, Kick green.
func platformDot(platform string) string {
	switch platform {
	case "twitch":
		return "🟣"
	case "kick":
		return "🟢"
	}
	return "⚪"
}

// formatMinutes renders minutes as "20h 34m", or "45m" under an hour.
func formatMinutes(m int) string {
	if m < 60 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %02dm", m/60, m%60)
}
