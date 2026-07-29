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

func main() {
	today := time.Now().Truncate(24 * time.Hour)
	startFlag := flag.String("start", today.AddDate(0, 0, -30).Format(time.DateOnly), "date de début incluse (AAAA-MM-JJ)")
	endFlag := flag.String("end", today.Format(time.DateOnly), "date de fin exclue (AAAA-MM-JJ)")
	dbFlag := flag.String("db", "data/conso.duckdb", "chemin du fichier DuckDB")
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
	if err := os.MkdirAll(filepath.Dir(*dbFlag), 0o755); err != nil {
		log.Fatalf("création du dossier de données: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := client.DailyConsumption(ctx, start, end)
	if err != nil {
		log.Fatal(err)
	}

	store, err := storage.Open(*dbFlag)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	count, err := store.UpsertDailyConsumption(ctx, prm, result.Quality, result.IntervalReading)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d relevé(s) enregistré(s) dans %s\n", count, *dbFlag)
}
