package utils

import (
	"fmt"
	"math"
	"strings"
	"time"
)

func FormatTimestamp(t time.Time) string {
	now := time.Now()
	if IsSameDay(t, now) {
		return "Today at " + t.Format("3:04 PM")
	}
	yesterday := now.AddDate(0, 0, -1)
	if IsSameDay(t, yesterday) {
		return "Yesterday at " + t.Format("3:04 PM")
	}
	return t.Format("01/02/2006 3:04 PM")
}

func FormatTimestampShort(t time.Time) string {
	return t.Format("3:04 PM")
}

func FormatDate(t time.Time) string {
	now := time.Now()
	if IsSameDay(t, now) {
		return "Today"
	}
	yesterday := now.AddDate(0, 0, -1)
	if IsSameDay(t, yesterday) {
		return "Yesterday"
	}
	return t.Format("January 2, 2006")
}

func IsSameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func WithinMinutes(a, b time.Time, minutes float64) bool {
	return math.Abs(a.Sub(b).Minutes()) <= minutes
}

func FormatCount(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func Pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func FormatTypingNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	case 3:
		return names[0] + ", " + names[1] + ", and " + names[2]
	default:
		return strings.Join(names[:3], ", ") + fmt.Sprintf(", and %d others", len(names)-3)
	}
}
