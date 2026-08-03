package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tomgachet/conso-dashboard/internal/conso"
)

func TestUpsertConsumptionLoadCurveIsIdempotent(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "conso.duckdb"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	first := conso.Reading{Date: "2026-07-01 00:30:00", Value: "5466", IntervalLength: "PT30M", MeasureType: "B"}
	updated := conso.Reading{Date: first.Date, Value: "5500", IntervalLength: "PT30M", MeasureType: "B"}
	if _, err := store.UpsertConsumptionLoadCurve(ctx, "12345678901234", "BRUT", []conso.Reading{first}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertConsumptionLoadCurve(ctx, "12345678901234", "CORRIGE", []conso.Reading{updated}); err != nil {
		t.Fatal(err)
	}

	var count, valueW int64
	if err := store.db.QueryRowContext(ctx, `SELECT count(*), max(value_w) FROM consumption_load_curve`).Scan(&count, &valueW); err != nil {
		t.Fatal(err)
	}
	if count != 1 || valueW != 5500 {
		t.Fatalf("count=%d value_w=%d", count, valueW)
	}
}

func TestDailyConsumptionUsesCalendarDateBoundary(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "conso.duckdb"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	readings := []conso.Reading{
		{Date: "2026-07-31 12:00:00", Value: "1000", IntervalLength: "PT30M", MeasureType: "B"},
		{Date: "2026-08-01 12:00:00", Value: "2000", IntervalLength: "PT30M", MeasureType: "B"},
	}
	if _, err := store.UpsertConsumptionLoadCurve(ctx, "12345678901234", "BRUT", readings); err != nil {
		t.Fatal(err)
	}

	paris := time.FixedZone("Europe/Paris", 2*60*60)
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, paris)
	points, err := store.DailyConsumption(ctx, start)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Day.Format(time.DateOnly) != "2026-08-01" {
		t.Fatalf("points = %#v", points)
	}
}
