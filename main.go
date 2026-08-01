package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/tomgachet/conso-dashboard/internal/conso"
	"github.com/tomgachet/conso-dashboard/internal/storage"
)

var version = "dev"

func main() {
	args := os.Args[1:]
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("conso-dashboard %s\n", version)
			return
		case "serve":
			if err := runServer(os.Args[2:]); err != nil {
				log.Fatal(err)
			}
			return
		case "fetch":
			var err error
			args, err = fetchArgs(os.Args[2:], time.Now())
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	if err := runImport(args, time.Now()); err != nil {
		log.Fatal(err)
	}
}

func runImport(args []string, now time.Time) error {
	const dbPath = "data/conso.duckdb"
	if err := loadEnvFile(".env"); err != nil {
		return err
	}

	today := localDate(now)
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	startFlag := flags.String("start", today.AddDate(0, 0, -30).Format(time.DateOnly), "date de début incluse (AAAA-MM-JJ)")
	endFlag := flags.String("end", today.Format(time.DateOnly), "date de fin exclue (AAAA-MM-JJ)")
	if err := flags.Parse(args); err != nil {
		return err
	}

	start, err := time.Parse(time.DateOnly, *startFlag)
	if err != nil {
		return fmt.Errorf("date de début invalide: %w", err)
	}
	end, err := time.Parse(time.DateOnly, *endFlag)
	if err != nil {
		return fmt.Errorf("date de fin invalide: %w", err)
	}
	if !start.Before(end) {
		return fmt.Errorf("la date de début doit précéder la date de fin")
	}

	prm := os.Getenv("CONSO_API_PRM")
	client, err := conso.NewClient(os.Getenv("CONSO_API_BASE_URL"), os.Getenv("CONSO_API_TOKEN"), prm, nil)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("création du dossier de données: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	store, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		return err
	}
	var count int
	for _, period := range splitPeriod(start, end, 6) {
		result, err := client.Consumption(ctx, period.start, period.end)
		if err != nil {
			return err
		}
		stored, err := store.UpsertConsumptionLoadCurve(ctx, prm, result.Quality, result.IntervalReading)
		if err != nil {
			return err
		}
		count += stored
	}
	fmt.Printf("%d relevé(s) enregistré(s) dans %s\n", count, dbPath)
	return nil
}

func localDate(now time.Time) time.Time {
	year, month, day := now.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, now.Location())
}

func fetchArgs(args []string, now time.Time) ([]string, error) {
	if len(args) != 1 || args[0] != "yesterday" {
		return nil, fmt.Errorf("utilisation: conso-dashboard fetch yesterday")
	}
	today := localDate(now)
	return []string{
		"-start", today.AddDate(0, 0, -1).Format(time.DateOnly),
		"-end", today.Format(time.DateOnly),
	}, nil
}

type period struct {
	start time.Time
	end   time.Time
}

func splitPeriod(start, end time.Time, maxDays int) []period {
	var periods []period
	for cursor := start; cursor.Before(end); {
		chunkEnd := cursor.AddDate(0, 0, maxDays)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		periods = append(periods, period{start: cursor, end: chunkEnd})
		cursor = chunkEnd
	}
	return periods
}
