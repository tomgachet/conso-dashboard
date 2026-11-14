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
		if r.URL.Path != "/api/courbe_de_charge_consommation" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		for key, want := range map[string]string{"pointId": "12345678901234", "dateDebut": "2026-07-01", "dateFin": "2026-07-02"} {
			if got := r.URL.Query().Get(key); got != want {
				t.Errorf("%s = %q, attendu %q", key, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"grandeur":[{"grandeurMetier":"CONS","grandeurPhysique":"PA","unite":"W","points":[{"d":"2026-07-01 00:15:00","v":"5466","p":"PT15M","n":"B","iv":"0","ec":"0"},{"d":"2026-07-01 00:45:00","v":"1200","p":"PT30M","n":"B"}]}]}`))
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
	if len(result.IntervalReading) != 2 || result.IntervalReading[0] != (Reading{Date: "2026-07-01 00:15:00", Value: "5466", IntervalLength: "PT15M", MeasureType: "B"}) || result.IntervalReading[1].IntervalLength != "PT30M" || result.Quality != "" {
		t.Fatalf("réponse inattendue: %#v", result)
	}
}

func TestNewClientRejectsInvalidPRM(t *testing.T) {
	if _, err := NewClient("https://example.com", "secret", "123", nil); err == nil {
		t.Fatal("un PRM invalide devrait être refusé")
	}
}

func TestConsumptionResponseValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		wantError bool
	}{
		{"ancien format", `{"quality":"BRUT","interval_reading":[]}`, true},
		{"série absente", `{"grandeur":[]}`, true},
		{"mauvaise unité", `{"grandeur":[{"grandeurMetier":"CONS","grandeurPhysique":"PA","unite":"kW","points":[]}]}`, true},
		{"points absents", `{"grandeur":[{"grandeurMetier":"CONS","grandeurPhysique":"PA","unite":"W"}]}`, true},
		{"intervalle inconnu", `{"grandeur":[{"grandeurMetier":"CONS","grandeurPhysique":"PA","unite":"W","points":[{"d":"2026-07-01 00:30:00","v":"10","p":"PT60M"}]}]}`, true},
		{"date invalide", `{"grandeur":[{"grandeurMetier":"CONS","grandeurPhysique":"PA","unite":"W","points":[{"d":"invalide","v":"10","p":"PT30M"}]}]}`, true},
		{"JSON invalide", `{`, true},
		{"série vide", `{"grandeur":[{"grandeurMetier":"CONS","grandeurPhysique":"PA","unite":"W","points":[]}]}`, false},
		{"série dupliquée", `{"grandeur":[{"grandeurMetier":"CONS","grandeurPhysique":"PA","unite":"W","points":[]},{"grandeurMetier":"CONS","grandeurPhysique":"PA","unite":"W","points":[]}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client, err := NewClient(server.URL, "secret", "12345678901234", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Consumption(context.Background(), date(2026, 7, 1), date(2026, 7, 2))
			if (err != nil) != tc.wantError {
				t.Fatalf("erreur = %v, erreur attendue = %v", err, tc.wantError)
			}
		})
	}
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
