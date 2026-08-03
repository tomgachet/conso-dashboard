package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tomgachet/conso-dashboard/internal/storage"
)

type fakeDailyReader struct {
	start  time.Time
	points []storage.DailyConsumption
}

func (f *fakeDailyReader) PRM(_ context.Context) (string, error) {
	return "12345678901234", nil
}

func (f *fakeDailyReader) DailyConsumption(_ context.Context, start time.Time) ([]storage.DailyConsumption, error) {
	f.start = start
	if f.points != nil {
		return f.points, nil
	}
	return []storage.DailyConsumption{{Day: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC), KWh: 21.7}}, nil
}

func (f *fakeDailyReader) IntervalConsumption(_ context.Context, _ time.Time) ([]storage.IntervalConsumption, error) {
	return []storage.IntervalConsumption{{Time: time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC), KWh: 0.125}}, nil
}

func TestDailyHandler(t *testing.T) {
	reader := &fakeDailyReader{}
	recorder := httptest.NewRecorder()
	dailyHandler(reader).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/daily?period=week", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if reader.start.Weekday() != time.Monday {
		t.Fatalf("start = %s, attendu un lundi", reader.start)
	}
	if got := recorder.Body.String(); got != "[{\"day\":\"2026-07-28\",\"kwh\":21.7}]\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestInfoHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	infoHandler(&fakeDailyReader{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/info", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Body.String(); got != "{\"prm\":\"12345678901234\"}\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestDailyHandlerRejectsInvalidPeriod(t *testing.T) {
	recorder := httptest.NewRecorder()
	dailyHandler(&fakeDailyReader{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/daily?period=quarter", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestDailyHandlerFiltersSelectedYear(t *testing.T) {
	reader := &fakeDailyReader{points: []storage.DailyConsumption{
		{Day: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), KWh: 12},
		{Day: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), KWh: 13},
		{Day: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), KWh: 14},
	}}
	recorder := httptest.NewRecorder()
	dailyHandler(reader).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/daily?year=2026", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := reader.start.Format(time.DateOnly); got != "2026-01-01" {
		t.Fatalf("start = %s", got)
	}
	if got := recorder.Body.String(); got != "[{\"day\":\"2026-01-01\",\"kwh\":13}]\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestDailyHandlerRejectsInvalidYear(t *testing.T) {
	recorder := httptest.NewRecorder()
	dailyHandler(&fakeDailyReader{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/daily?year=hier", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestPeriodStart(t *testing.T) {
	paris := time.FixedZone("Europe/Paris", 2*60*60)
	now := time.Date(2026, 8, 2, 14, 30, 0, 0, paris)
	tests := map[string]string{"week": "2026-07-27", "month": "2026-08-01", "year": "2026-01-01"}
	for period, want := range tests {
		start, ok := periodStart(now, period)
		if !ok || start.Format(time.DateOnly) != want {
			t.Errorf("periodStart(%q) = %s, %v", period, start, ok)
		}
	}
}

func TestIntervalHandler(t *testing.T) {
	recorder := httptest.NewRecorder()
	intervalHandler(&fakeDailyReader{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/intervals?day=2026-07-28", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Body.String(); got != "[{\"time\":\"24:00\",\"kwh\":0.125}]\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestIntervalHandlerRejectsInvalidDay(t *testing.T) {
	recorder := httptest.NewRecorder()
	intervalHandler(&fakeDailyReader{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/intervals?day=non", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}
