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
	days int
}

func (f *fakeDailyReader) PRM(_ context.Context) (string, error) {
	return "12345678901234", nil
}

func (f *fakeDailyReader) DailyConsumption(_ context.Context, days int) ([]storage.DailyConsumption, error) {
	f.days = days
	return []storage.DailyConsumption{{Day: time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC), KWh: 21.7}}, nil
}

func (f *fakeDailyReader) IntervalConsumption(_ context.Context, _ time.Time) ([]storage.IntervalConsumption, error) {
	return []storage.IntervalConsumption{{Time: time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC), KWh: 0.125}}, nil
}

func TestDailyHandler(t *testing.T) {
	reader := &fakeDailyReader{}
	recorder := httptest.NewRecorder()
	dailyHandler(reader).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/daily?days=7", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if reader.days != 7 {
		t.Fatalf("days = %d", reader.days)
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
	dailyHandler(&fakeDailyReader{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/daily?days=8", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
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
