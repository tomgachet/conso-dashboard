package conso

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConsumption(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/consumption_load_curve" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.URL.Query().Get("start"); got != "2026-07-01" {
			t.Errorf("start = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"quality":"BRUT","interval_reading":[{"date":"2026-07-01 00:30:00","value":"5466","interval_length":"PT30M","measure_type":"B"}]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "secret", "12345678901234", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Consumption(context.Background(), date(2026, 7, 1), date(2026, 7, 2))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.IntervalReading) != 1 || result.IntervalReading[0].IntervalLength != "PT30M" {
		t.Fatalf("réponse inattendue: %#v", result)
	}
}

func TestNewClientRejectsInvalidPRM(t *testing.T) {
	if _, err := NewClient("https://example.com", "secret", "123", nil); err == nil {
		t.Fatal("un PRM invalide devrait être refusé")
	}
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
