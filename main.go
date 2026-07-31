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
		}
	}

	const dbPath = "data/conso.duckdb"
	if err := loadEnvFile(".env"); err != nil {
		log.Fatal(err)
	}

	today := time.Now().Truncate(24 * time.Hour)
	startFlag := flag.String("start", today.AddDate(0, 0, -30).Format(time.DateOnly), "date de début incluse (AAAA-MM-JJ)")
	endFlag := flag.String("end", today.Format(time.DateOnly), "date de fin exclue (AAAA-MM-JJ)")
	flag.Parse()

	start, err := time.Parse(time.DateOnly, *startFlag)
	if err != nil {
		log.Fatalf("date de début invalide: %v", err)
	}
	end, err := time.Parse(time.DateOnly, *endFlag)
	if err != nil {
		log.Fatalf("date de fin invalide: %v", err)
	}
	if !start.Before(end) {
		log.Fatal("la date de début doit précéder la date de fin")
	}

	prm := os.Getenv("CONSO_API_PRM")
	client, err := conso.NewClient(os.Getenv("CONSO_API_BASE_URL"), os.Getenv("CONSO_API_TOKEN"), prm, nil)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("création du dossier de données: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	store, err := storage.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	var count int
	for _, period := range splitPeriod(start, end, 6) {
		result, err := client.Consumption(ctx, period.start, period.end)
		if err != nil {
			log.Fatal(err)
		}
		stored, err := store.UpsertConsumptionLoadCurve(ctx, prm, result.Quality, result.IntervalReading)
		if err != nil {
			log.Fatal(err)
		}
		count += stored
	}
	fmt.Printf("%d relevé(s) enregistré(s) dans %s\n", count, dbPath)
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
