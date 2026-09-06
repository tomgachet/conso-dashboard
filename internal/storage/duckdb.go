package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"

	"github.com/tomgachet/conso-dashboard/internal/conso"
)

type Store struct {
	db *sql.DB
}

type DailyConsumption struct {
	Day time.Time
	KWh float64
}

type IntervalConsumption struct {
	Time time.Time
	KWh  float64
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("ouverture de DuckDB: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connexion à DuckDB: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) PRM(ctx context.Context) (string, error) {
	var prm string
	if err := s.db.QueryRowContext(ctx, `SELECT prm FROM consumption_load_curve LIMIT 1`).Scan(&prm); err != nil {
		return "", fmt.Errorf("lecture du PRM: %w", err)
	}
	return prm, nil
}

func (s *Store) DailyConsumption(ctx context.Context, start time.Time) ([]DailyConsumption, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH daily AS (
			SELECT
				CAST(reading_at - INTERVAL 1 MICROSECOND AS DATE) AS day,
				SUM(value_w * CASE interval_length
					WHEN 'PT15M' THEN 0.25
					WHEN 'PT30M' THEN 0.5
					ELSE 0
				END) / 1000 AS consumption_kwh
			FROM consumption_load_curve
			GROUP BY day
		)
		SELECT day, consumption_kwh
		FROM daily
		WHERE day >= CAST(? AS DATE)
		ORDER BY day
	`, start.Format(time.DateOnly))
	if err != nil {
		return nil, fmt.Errorf("lecture des consommations quotidiennes: %w", err)
	}
	defer rows.Close()

	var result []DailyConsumption
	for rows.Next() {
		var point DailyConsumption
		if err := rows.Scan(&point.Day, &point.KWh); err != nil {
			return nil, fmt.Errorf("lecture d'une consommation quotidienne: %w", err)
		}
		result = append(result, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lecture des consommations quotidiennes: %w", err)
	}
	return result, nil
}

func (s *Store) IntervalConsumption(ctx context.Context, day time.Time) ([]IntervalConsumption, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			reading_at,
			value_w * CASE interval_length
				WHEN 'PT15M' THEN 0.25
				WHEN 'PT30M' THEN 0.5
				ELSE 0
			END / 1000 AS consumption_kwh
		FROM consumption_load_curve
		WHERE CAST(reading_at - INTERVAL 1 MICROSECOND AS DATE) = ?
		ORDER BY reading_at
	`, day)
	if err != nil {
		return nil, fmt.Errorf("lecture du détail de consommation: %w", err)
	}
	defer rows.Close()

	var result []IntervalConsumption
	for rows.Next() {
		var point IntervalConsumption
		if err := rows.Scan(&point.Time, &point.KWh); err != nil {
			return nil, fmt.Errorf("lecture d'un détail de consommation: %w", err)
		}
		result = append(result, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lecture du détail de consommation: %w", err)
	}
	return result, nil
}

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS consumption_load_curve (
			prm VARCHAR NOT NULL,
			reading_at TIMESTAMP NOT NULL,
			value_w BIGINT NOT NULL,
			interval_length VARCHAR NOT NULL,
			measure_type VARCHAR,
			quality VARCHAR,
			fetched_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (prm, reading_at)
		);
	`)
	if err != nil {
		return fmt.Errorf("création du schéma: %w", err)
	}
	return nil
}

type ImportStats struct {
	Inserted int
	Existing int
}

func (s *Store) UpsertConsumptionLoadCurve(ctx context.Context, prm, quality string, readings []conso.Reading) (ImportStats, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportStats{}, fmt.Errorf("début de transaction: %w", err)
	}
	defer tx.Rollback()

	stats := ImportStats{}
	for _, reading := range readings {
		if _, err := time.Parse("2006-01-02 15:04:05", reading.Date); err != nil {
			return ImportStats{}, fmt.Errorf("horodatage %q invalide: %w", reading.Date, err)
		}
		valueW, err := strconv.ParseInt(reading.Value, 10, 64)
		if err != nil {
			return ImportStats{}, fmt.Errorf("valeur %q invalide pour %s: %w", reading.Value, reading.Date, err)
		}
		if reading.IntervalLength == "" {
			return ImportStats{}, fmt.Errorf("durée d'intervalle absente pour %s", reading.Date)
		}
		var exists bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM consumption_load_curve
			WHERE prm = ? AND reading_at = CAST(? AS TIMESTAMP))
		`, prm, reading.Date).Scan(&exists); err != nil {
			return ImportStats{}, fmt.Errorf("recherche de %s: %w", reading.Date, err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO consumption_load_curve
				(prm, reading_at, value_w, interval_length, measure_type, quality, fetched_at)
			VALUES (?, CAST(? AS TIMESTAMP), ?, ?, ?, ?, current_timestamp)
			ON CONFLICT (prm, reading_at) DO UPDATE SET
				value_w = excluded.value_w,
				interval_length = excluded.interval_length,
				measure_type = excluded.measure_type,
				quality = excluded.quality,
				fetched_at = excluded.fetched_at
		`, prm, reading.Date, valueW, reading.IntervalLength, reading.MeasureType, quality)
		if err != nil {
			return ImportStats{}, fmt.Errorf("enregistrement de %s: %w", reading.Date, err)
		}
		if exists {
			stats.Existing++
		} else {
			stats.Inserted++
		}
	}
	if err := tx.Commit(); err != nil {
		return ImportStats{}, fmt.Errorf("validation de la transaction: %w", err)
	}
	return stats, nil
}
