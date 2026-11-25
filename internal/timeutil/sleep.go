package timeutil

import (
	"fmt"
	"hash/fnv"
	"time"
)

// SleepJitter is the default bound of the daily random shift of the sleep
// window's start and end, each way.
const SleepJitter = 20 * time.Minute

// MaxSleepJitter is the largest shift the setting allows. SleepUntil's
// look-back assumes it stays well under a day.
const MaxSleepJitter = 120 * time.Minute

// minSleepSpan is the shortest a jittered window may get. A window set
// shorter than this keeps its set length instead.
const minSleepSpan = 30 * time.Minute

// ParseClock parses a strict "HH:MM" 24-hour clock time into minutes after
// midnight.
func ParseClock(s string) (int, bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, false
	}
	digit := func(c byte) (int, bool) { return int(c - '0'), c >= '0' && c <= '9' }
	h1, ok1 := digit(s[0])
	h2, ok2 := digit(s[1])
	m1, ok3 := digit(s[3])
	m2, ok4 := digit(s[4])
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return 0, false
	}
	h, m := h1*10+h2, m1*10+m2
	if h > 23 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// FormatClock formats minutes after midnight as "HH:MM".
func FormatClock(min int) string {
	min = ((min % 1440) + 1440) % 1440
	return fmt.Sprintf("%02d:%02d", min/60, min%60)
}

// SleepUntil reports whether now falls inside the daily sleep window, read
// in loc, and if so when it ends. start and end are minutes after midnight;
// an end at or before start crosses midnight. start == end never sleeps.
//
// With jitter > 0, each window's start and end shift by up to jitter (capped
// at MaxSleepJitter), keyed on seed and the calendar date the window starts
// on, so the wake time stays the same all day. Different seeds (installs,
// accounts) shift differently. Windows that overlap under jitter merge, so
// the returned wake is never itself inside a window.
func SleepUntil(now time.Time, loc *time.Location, start, end int, jitter time.Duration, seed string) (wake time.Time, in bool) {
	if loc == nil {
		loc = time.UTC
	}
	if start == end || start < 0 || end < 0 || start >= 1440 || end >= 1440 {
		return time.Time{}, false
	}
	jitter = min(max(jitter, 0), MaxSleepJitter)
	y, m, d := now.In(loc).Date()
	// A window can start up to two calendar days back (near-24h window plus
	// jitter) or on the next day (a start just after midnight, jittered
	// back before it).
	for off := -2; off <= 1; off++ {
		s, e := sleepBounds(y, m, d+off, loc, start, end, jitter, seed)
		if now.Before(s) || !now.Before(e) {
			continue
		}
		for next := off + 1; next <= off+3; next++ {
			ns, ne := sleepBounds(y, m, d+next, loc, start, end, jitter, seed)
			if ns.After(e) {
				break
			}
			if ne.After(e) {
				e = ne
			}
		}
		return e, true
	}
	return time.Time{}, false
}

// sleepBounds is the window that starts on calendar day y-m-d in loc
// (normalized like time.Date). It is empty (s == e) when a DST jump leaves
// no time between the set start and end.
func sleepBounds(y int, m time.Month, d int, loc *time.Location, start, end int, jitter time.Duration, seed string) (s, e time.Time) {
	s = time.Date(y, m, d, start/60, start%60, 0, 0, loc)
	endDay := d
	if end <= start {
		endDay++
	}
	e = time.Date(y, m, endDay, end/60, end%60, 0, 0, loc)
	span := e.Sub(s)
	if span <= 0 {
		return s, s
	}
	// Keep a third of the awake gap free of shift, so one night's end never
	// reaches the next night's start and the account always wakes each day.
	if gap := (24*time.Hour - span) / 3; jitter > gap {
		jitter = gap
	}
	if jitter < time.Minute {
		return s, e
	}
	day := time.Date(y, m, d, 12, 0, 0, 0, loc)
	s = s.Add(sleepJitter(day, seed, "start", jitter))
	e = e.Add(sleepJitter(day, seed, "end", jitter))
	if floor := min(span, minSleepSpan); e.Sub(s) < floor {
		e = s.Add(floor)
	}
	return s, e
}

// sleepJitter is the shift, in whole minutes within ±bound, for the window
// starting on day's calendar date under seed. tag tells the start and end
// apart.
func sleepJitter(day time.Time, seed, tag string, bound time.Duration) time.Duration {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed + "|" + day.Format(time.DateOnly) + "|" + tag))
	n := int64(bound / time.Minute)
	return time.Duration(int64(h.Sum32())%(2*n+1)-n) * time.Minute
}
