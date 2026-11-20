package notify

import (
	"fmt"
	"strings"
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
	embed["description"] = fmt.Sprintf("📅 **%s**\n⏱ **%s** watched · 🎁 **%d** %s claimed",
		r.Day.Format("Monday, 2 January 2006"), formatMinutes(r.Minutes), len(r.Drops), plural(len(r.Drops), "drop", "drops"))
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
	add("⏱ Watch time", watch)

	if len(r.Drops) == 0 {
		add("🎁 Drops claimed", []string{"None today"})
	} else {
		manyAccounts := len(distinct(r.Drops, func(d SummaryDrop) string { return d.Account })) > 1
		groups := groupDrops(r.Drops)
		for gi, g := range groups {
			game := g.game
			if game == "" {
				game = "Other"
			}
			lines := make([]string, 0, len(g.drops))
			for i, d := range g.drops {
				if i == maxSummaryDropsPerGame {
					lines = append(lines, fmt.Sprintf("*+%d more*", len(g.drops)-i))
					break
				}
				item := d.Drop
				if item == "" {
					item = "Drop"
				}
				line := "• " + mdEscape(item)
				if manyAccounts && d.Account != "" {
					line += " *· " + mdEscape(d.Account) + "*"
				}
				lines = append(lines, line)
			}
			// Keep the last field for the "more games" note.
			last := len(fields) == discordMaxFields-1 && gi < len(groups)-1
			if last || !add(fmt.Sprintf("🎮 %s %s (%d)", game, platformDot(g.platform), len(g.drops)), lines) {
				add("…", []string{fmt.Sprintf("*+%d more games*", len(groups)-gi)})
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

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
