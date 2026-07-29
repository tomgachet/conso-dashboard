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
	"strconv"
	"strings"
	"time"

	"github.com/tomgachet/conso-dashboard/internal/storage"
)

//go:embed web/index.html
var dashboardFiles embed.FS

type dailyReader interface {
	DailyConsumption(context.Context, int) ([]storage.DailyConsumption, error)
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
	mux.HandleFunc("GET /api/daily", dailyHandler(store))

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

func dailyHandler(reader dailyReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		days := 30
		if value := r.URL.Query().Get("days"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || (parsed != 7 && parsed != 30 && parsed != 90) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "days doit valoir 7, 30 ou 90"})
				return
			}
			days = parsed
		}
		points, err := reader.DailyConsumption(r.Context(), days)
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
