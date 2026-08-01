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

func TestFetchYesterdayArgs(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 1, 0, 30, 0, 0, paris)

	args, err := fetchArgs([]string{"yesterday"}, now)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-start", "2026-07-31", "-end", "2026-08-01"}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("argument %d = %q, attendu %q", i, args[i], want[i])
		}
	}
}

func TestFetchArgsRejectsExtraArgument(t *testing.T) {
	if _, err := fetchArgs([]string{"yesterday", "extra"}, time.Now()); err == nil {
		t.Fatal("un argument supplémentaire devrait être refusé")
	}
}

func TestFetchArgsRejectsUnknownPeriod(t *testing.T) {
	if _, err := fetchArgs([]string{"tomorrow"}, time.Now()); err == nil {
		t.Fatal("une période inconnue devrait être refusée")
	}
}

func TestFetchArgsKeepsDateFlags(t *testing.T) {
	want := []string{"-start", "2026-07-01", "-end", "2026-07-02"}
	got, err := fetchArgs(want, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argument %d = %q, attendu %q", i, got[i], want[i])
		}
	}
}
