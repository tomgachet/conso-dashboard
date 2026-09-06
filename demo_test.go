package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tomgachet/conso-dashboard/internal/storage"
)

func TestDemoIsolatedAndQueryable(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("data", 0755); err != nil {
		t.Fatal(err)
	}
	// Invalid files would break a demo that accidentally opened the real database or .env.
	for _, path := range []string{".env", "data/conso.duckdb"} {
		if err := os.WriteFile(path, []byte("do not touch"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	store, err := storage.NewDemo(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	points, err := store.DailyConsumption(ctx, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 366 {
		t.Fatalf("got %d days, want 366", len(points))
	}
	day := localDate(now)
	intervals, err := store.IntervalConsumption(ctx, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(intervals) != 96 {
		t.Fatalf("got %d intervals, want 96", len(intervals))
	}
	if !intervals[0].Time.Equal(day.Add(15*time.Minute)) || !intervals[95].Time.Equal(day.AddDate(0, 0, 1)) {
		t.Fatal("incorrect interval endpoints")
	}
	var total float64
	for _, p := range intervals {
		if p.KWh <= 0 {
			t.Fatal("non-positive consumption")
		}
		total += p.KWh
	}
	if diff := total - points[len(points)-1].KWh; diff < -0.000001 || diff > 0.000001 {
		t.Fatal("daily and interval totals differ")
	}
	recorder := httptest.NewRecorder()
	infoHandlerMode(store, true).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/info", nil))
	var info struct {
		PRM  string
		Demo bool
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != 200 || !info.Demo || info.PRM != "00000000000000" {
		t.Fatalf("unexpected info: %s", recorder.Body.String())
	}
	for _, path := range []string{".env", filepath.Join("data", "conso.duckdb")} {
		content, err := os.ReadFile(path)
		if err != nil || string(content) != "do not touch" {
			t.Fatalf("real file changed: %s", path)
		}
	}
}

func TestDemoRejectsUnexpectedArguments(t *testing.T) {
	if err := runDemo([]string{"unexpected"}); err == nil {
		t.Fatal("expected error")
	}
}
