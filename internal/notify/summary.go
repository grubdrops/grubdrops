package notify

import "time"

// FieldReport carries a *SummaryReport on EventSummary. Notifiers that know
// it render the day in detail; the others keep using title and msg.
const FieldReport = "report"

// SummaryReport is the day a daily summary covers: watch time per account
// and the drops claimed, oldest first.
type SummaryReport struct {
	Day      time.Time
	Lang     string // UI language for the labels; empty is English
	Minutes  int
	Accounts []SummaryAccount
	Drops    []SummaryDrop
}

// SummaryAccount is one account's watch time on the day.
type SummaryAccount struct {
	Label, Platform string
	Minutes         int
}

// SummaryDrop is one drop claimed on the day.
type SummaryDrop struct {
	Game, Drop, Platform, Account string
}
