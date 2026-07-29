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

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS daily_consumption (
			prm VARCHAR NOT NULL,
			reading_date DATE NOT NULL,
			value_wh BIGINT NOT NULL,
			quality VARCHAR,
			fetched_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (prm, reading_date)
		)
	`)
	if err != nil {
		return fmt.Errorf("création du schéma: %w", err)
	}
	return nil
}

func (s *Store) UpsertDailyConsumption(ctx context.Context, prm, quality string, readings []conso.Reading) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("début de transaction: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, reading := range readings {
		day, err := time.Parse(time.DateOnly, reading.Date)
		if err != nil {
			return 0, fmt.Errorf("date %q invalide: %w", reading.Date, err)
		}
		valueWh, err := strconv.ParseInt(reading.Value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("valeur %q invalide pour le %s: %w", reading.Value, reading.Date, err)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO daily_consumption (prm, reading_date, value_wh, quality, fetched_at)
			VALUES (?, ?, ?, ?, current_timestamp)
			ON CONFLICT (prm, reading_date) DO UPDATE SET
				value_wh = excluded.value_wh,
				quality = excluded.quality,
				fetched_at = excluded.fetched_at
		`, prm, day, valueWh, quality)
		if err != nil {
			return 0, fmt.Errorf("enregistrement du %s: %w", reading.Date, err)
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("validation de la transaction: %w", err)
	}
	return count, nil
}
