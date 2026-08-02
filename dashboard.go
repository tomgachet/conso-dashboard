package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/tomgachet/conso-dashboard/internal/storage"
)

//go:embed web/index.html
var dashboardFiles embed.FS

type dailyReader interface {
	PRM(context.Context) (string, error)
	DailyConsumption(context.Context, time.Time) ([]storage.DailyConsumption, error)
	IntervalConsumption(context.Context, time.Time) ([]storage.IntervalConsumption, error)
}

func runServer(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := flags.String("addr", ":8080", "adresse d'écoute HTTP")
	if err := flags.Parse(args); err != nil {
		return err
	}

	store, err := storage.Open("data/conso.duckdb")
	if err != nil {
		return err
	}
	defer store.Close()

	static, err := fs.Sub(dashboardFiles, "web")
	if err != nil {
		return fmt.Errorf("chargement du dashboard: %w", err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(static))
	mux.HandleFunc("GET /api/info", infoHandler(store))
	mux.HandleFunc("GET /api/daily", dailyHandler(store))
	mux.HandleFunc("GET /api/intervals", intervalHandler(store))

	server := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	displayAddr := *addr
	if strings.HasPrefix(displayAddr, ":") {
		displayAddr = "localhost" + displayAddr
	}
	log.Printf("dashboard disponible sur http://%s", displayAddr)
	return server.ListenAndServe()
}

func infoHandler(reader dailyReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prm, err := reader.PRM(r.Context())
		if err != nil {
			log.Printf("dashboard: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "lecture de DuckDB impossible"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"prm": prm})
	}
}

func intervalHandler(reader dailyReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		day, err := time.Parse(time.DateOnly, r.URL.Query().Get("day"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "day doit être une date au format AAAA-MM-JJ"})
			return
		}
		points, err := reader.IntervalConsumption(r.Context(), day)
		if err != nil {
			log.Printf("dashboard: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "lecture de DuckDB impossible"})
			return
		}
		type responsePoint struct {
			Time string  `json:"time"`
			KWh  float64 `json:"kwh"`
		}
		response := make([]responsePoint, 0, len(points))
		for _, point := range points {
			label := point.Time.Format("15:04")
			if point.Time.Hour() == 0 && point.Time.Minute() == 0 && point.Time.Format(time.DateOnly) != day.Format(time.DateOnly) {
				label = "24:00"
			}
			response = append(response, responsePoint{Time: label, KWh: point.KWh})
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func dailyHandler(reader dailyReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		period := r.URL.Query().Get("period")
		if period == "" {
			period = "month"
		}
		start, ok := periodStart(time.Now(), period)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "period doit valoir week, month ou year"})
			return
		}
		points, err := reader.DailyConsumption(r.Context(), start)
		if err != nil {
			log.Printf("dashboard: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "lecture de DuckDB impossible"})
			return
		}
		type responsePoint struct {
			Day string  `json:"day"`
			KWh float64 `json:"kwh"`
		}
		response := make([]responsePoint, 0, len(points))
		for _, point := range points {
			response = append(response, responsePoint{Day: point.Day.Format(time.DateOnly), KWh: point.KWh})
		}
		writeJSON(w, http.StatusOK, response)
	}
}

func periodStart(now time.Time, period string) (time.Time, bool) {
	year, month, day := now.Date()
	start := time.Date(year, month, day, 0, 0, 0, 0, now.Location())
	switch period {
	case "week":
		start = start.AddDate(0, 0, -int((start.Weekday()+6)%7))
	case "month":
		start = time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
	case "year":
		start = time.Date(year, time.January, 1, 0, 0, 0, 0, now.Location())
	default:
		return time.Time{}, false
	}
	return start, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
