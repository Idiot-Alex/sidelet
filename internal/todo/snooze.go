package todo

import "time"

// SnoozeUntil is shared by the store and native UI. Tomorrow means 09:00
// in the caller's local timezone, including daylight-saving transitions.
func SnoozeUntil(now time.Time, duration string) (time.Time, bool) {
	switch duration {
	case "30m":
		return now.Add(30 * time.Minute), true
	case "1h":
		return now.Add(time.Hour), true
	case "tomorrow":
		next := now.AddDate(0, 0, 1)
		return time.Date(next.Year(), next.Month(), next.Day(), 9, 0, 0, 0, now.Location()), true
	default:
		return time.Time{}, false
	}
}
