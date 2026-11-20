package notify

import (
	"fmt"
	"strings"

	"github.com/grubdrops/grubdrops/internal/i18n"
)

// Discord embed limits the summary stays within.
const (
	discordMaxFields     = 25
	discordFieldValueMax = 1024
	discordFieldNameMax  = 256
	// discordEmbedBudget keeps the whole embed under Discord's 6000
	// characters with room for the title, description and footer.
	discordEmbedBudget = 5500
)

// summaryColor is the accent of a detailed daily summary.
const summaryColor = 0x3498DB

// discordSummary fills embed with a daily summary report: the day as
// description, a watch time field with one line per account, then one
// field per game with its drops.
func discordSummary(embed map[string]any, r *SummaryReport) map[string]any {
	embed["title"] = "📊 " + embed["title"].(string)
	embed["color"] = summaryColor
	claimed := fmt.Sprintf(i18n.T(r.Lang, "summary.claimed_other"), "**"+fmt.Sprint(len(r.Drops))+"**")
	if len(r.Drops) == 1 {
		claimed = fmt.Sprintf(i18n.T(r.Lang, "summary.claimed_one"), "**1**")
	}
	embed["description"] = fmt.Sprintf("📅 **%s**\n⏱ %s · 🎁 %s",
		r.Day.Format("2006-01-02"),
		fmt.Sprintf(i18n.T(r.Lang, "summary.watched"), "**"+formatMinutes(r.Minutes)+"**"), claimed)
	embed["footer"] = map[string]any{"text": "GrubDrops"}

	var fields []any
	used := 0
	add := func(name string, lines []string) bool {
		if len(fields) >= discordMaxFields {
			return false
		}
		name = clip(name, discordFieldNameMax)
		value := joinWithin(lines, discordFieldValueMax)
		if used+len(name)+len(value) > discordEmbedBudget {
			return false
		}
		used += len(name) + len(value)
		fields = append(fields, map[string]any{"name": name, "value": value, "inline": false})
		return true
	}

	watch := make([]string, 0, len(r.Accounts))
	for _, a := range r.Accounts {
		name := a.Label
		if p := platformLabel(a.Platform); p != "" {
			name = p + " · " + name
		}
		watch = append(watch, platformDot(a.Platform)+" "+mdEscape(name)+" — `"+formatMinutes(a.Minutes)+"`")
	}
	if len(watch) == 0 {
		watch = []string{"—"}
	}
	add("⏱ "+i18n.T(r.Lang, "summary.watch_time"), watch)

	if len(r.Drops) == 0 {
		add("🎁 "+i18n.T(r.Lang, "summary.drops_claimed"), []string{i18n.T(r.Lang, "summary.none_today")})
	} else {
		manyAccounts := len(distinct(r.Drops, func(d SummaryDrop) string { return d.Account })) > 1
		groups := groupDrops(r.Drops)
		for gi, g := range groups {
			game := g.game
			if game == "" {
				game = i18n.T(r.Lang, "summary.other_game")
			}
			lines := make([]string, 0, len(g.drops))
			for i, d := range g.drops {
				if i == maxSummaryDropsPerGame {
					lines = append(lines, "*"+fmt.Sprintf(i18n.T(r.Lang, "summary.more_drops"), len(g.drops)-i)+"*")
					break
				}
				item := d.Drop
				if item == "" {
					item = i18n.T(r.Lang, "summary.drop")
				}
				line := "• " + mdEscape(item)
				if manyAccounts && d.Account != "" {
					line += " *· " + mdEscape(d.Account) + "*"
				}
				lines = append(lines, line)
			}
			// Keep the last field for the "more games" note.
			last := len(fields) == discordMaxFields-1 && gi < len(groups)-1
			if last || !add(fmt.Sprintf("🎮 %s %s (%d)", mdEscape(game), platformDot(g.platform), len(g.drops)), lines) {
				add("…", []string{"*" + fmt.Sprintf(i18n.T(r.Lang, "summary.more_games"), len(groups)-gi) + "*"})
				break
			}
		}
	}
	embed["fields"] = fields
	return embed
}

// mdEscape escapes Discord markdown in text that is shown as is.
var mdEscaper = strings.NewReplacer(`\`, `\\`, `*`, `\*`, `_`, `\_`, "`", "\\`", `~`, `\~`, `|`, `\|`, `>`, `\>`)

func mdEscape(s string) string { return mdEscaper.Replace(s) }

// joinWithin joins lines with newlines, dropping whole lines past max and
// ending with "…" when it had to.
func joinWithin(lines []string, max int) string {
	var b strings.Builder
	for i, l := range lines {
		extra := len(l)
		if i > 0 {
			extra++
		}
		if b.Len()+extra > max-2 {
			if b.Len() == 0 {
				return clip(l, max)
			}
			b.WriteString("\n…")
			break
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l)
	}
	return b.String()
}

// clip cuts s to at most max bytes on a rune boundary.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	r := []rune(s)
	for len(string(r)) > max-3 {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
