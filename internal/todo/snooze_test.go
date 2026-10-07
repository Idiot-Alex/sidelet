package todo

import (
	"testing"
	"time"
)

func TestSnoozeUntilLocalTomorrowAcrossDST(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 7, 16, 0, 0, 0, zone)
	until, ok := SnoozeUntil(now, "tomorrow")
	if !ok || until.Day() != 8 || until.Hour() != 9 || until.Location() != zone || until.Sub(now) != 16*time.Hour {
		t.Fatalf("tomorrow must use local calendar: %s", until)
	}
	for _, tc := range []struct {
		duration string
		delta    time.Duration
	}{{"30m", 30 * time.Minute}, {"1h", time.Hour}} {
		got, ok := SnoozeUntil(now, tc.duration)
		if !ok || !got.Equal(now.Add(tc.delta)) {
			t.Fatal(tc.duration)
		}
	}
	if _, ok := SnoozeUntil(now, "invalid"); ok {
		t.Fatal("unknown duration accepted")
	}
}
