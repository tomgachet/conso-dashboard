package main

import (
	"testing"
	"time"
)

func TestSplitPeriod(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	periods := splitPeriod(start, end, 6)

	if len(periods) != 5 {
		t.Fatalf("nombre de périodes = %d", len(periods))
	}
	for _, period := range periods {
		if days := int(period.end.Sub(period.start).Hours() / 24); days > 6 {
			t.Fatalf("période trop longue: %d jours", days)
		}
	}
	if !periods[0].start.Equal(start) || !periods[len(periods)-1].end.Equal(end) {
		t.Fatalf("bornes inattendues: %#v", periods)
	}
}
