package storage

import (
	"context"
	"fmt"
	"time"
)

// NewDemo creates an isolated in-memory database. No configuration or data files are read.
func NewDemo(ctx context.Context, now time.Time) (*Store, error) {
	s, err := Open("")
	if err != nil {
		return nil, err
	}
	if err := s.Migrate(ctx); err != nil {
		s.Close()
		return nil, err
	}
	start := time.Date(now.Year()-1, time.January, 1, 0, 0, 0, 0, time.UTC)
	// Synthetic local wall-clock readings: 96 interval ends per complete day.
	// Include today so every period has data, even on January 1 or a Monday.
	end := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	_, err = s.db.ExecContext(ctx, `
 INSERT INTO consumption_load_curve
 SELECT '00000000000000', t + INTERVAL 15 MINUTE,
   CAST(180
     + 450 * (1 + cos(2 * pi() * (dayofyear(t) - 15) / 365.25))
     + 1100 * exp(-pow((hour(t) + minute(t) / 60.0 - 7.5) / 1.1, 2))
     + 1800 * exp(-pow((hour(t) + minute(t) / 60.0 - 19.5) / 1.8, 2))
     + CASE WHEN dayofweek(t) IN (0, 6) THEN 180 ELSE 0 END
     + 120 * (1 + sin(epoch(t) / 86400 * 1.7)) AS BIGINT),
   'PT15M', 'demo', 'demo', current_timestamp
 FROM generate_series(CAST(? AS TIMESTAMP), CAST(? AS TIMESTAMP) - INTERVAL 15 MINUTE, INTERVAL 15 MINUTE) AS slots(t)
 `, start.Format(time.DateTime), end.Format(time.DateTime))
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("création des données de démonstration: %w", err)
	}
	return s, nil
}
