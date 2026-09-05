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

func TestImportStatsCountOnlyCommittedReadings(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "conso.duckdb"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	first := conso.Reading{Date: "2026-07-01 00:15:00", Value: "100", IntervalLength: "PT15M"}
	second := conso.Reading{Date: "2026-07-01 00:30:00", Value: "200", IntervalLength: "PT15M"}
	third := conso.Reading{Date: "2026-07-01 00:45:00", Value: "300", IntervalLength: "PT15M"}
	invalid := conso.Reading{Date: "2026-07-01 01:00:00", Value: "invalid", IntervalLength: "PT15M"}
	for _, tc := range []struct {
		name      string
		prm       string
		readings  []conso.Reading
		want      ImportStats
		wantError bool
	}{
		{"first import", "123", []conso.Reading{first}, ImportStats{Inserted: 1}, false},
		{"repeat", "123", []conso.Reading{first}, ImportStats{Existing: 1}, false},
		{"mixed and duplicate", "123", []conso.Reading{first, second, second}, ImportStats{Inserted: 1, Existing: 2}, false},
		{"different meter", "456", []conso.Reading{first}, ImportStats{Inserted: 1}, false},
		{"empty", "123", nil, ImportStats{}, false},
		{"rollback", "123", []conso.Reading{third, invalid}, ImportStats{}, true},
		{"retry after rollback", "123", []conso.Reading{third}, ImportStats{Inserted: 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.UpsertConsumptionLoadCurve(ctx, tc.prm, "BRUT", tc.readings)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("stats = %+v, want %+v", got, tc.want)
			}
		})
	}
}
