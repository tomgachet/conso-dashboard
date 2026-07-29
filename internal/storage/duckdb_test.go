package storage

import (
	"context"
	"path/filepath"
	"testing"

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
