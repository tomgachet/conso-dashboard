package conso

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Reading struct {
	Date           string `json:"date"`
	Value          string `json:"value"`
	IntervalLength string `json:"interval_length"`
	MeasureType    string `json:"measure_type"`
}

type Consumption struct {
	Quality         string    `json:"quality"`
	IntervalReading []Reading `json:"interval_reading"`
}

type Client struct {
	baseURL    string
	token      string
	prm        string
	httpClient *http.Client
}

func NewClient(baseURL, token, prm string, httpClient *http.Client) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("CONSO_API_TOKEN est obligatoire")
	}
	if len(prm) != 14 {
		return nil, fmt.Errorf("CONSO_API_PRM doit contenir 14 chiffres")
	}
	for _, r := range prm {
		if r < '0' || r > '9' {
			return nil, fmt.Errorf("CONSO_API_PRM doit contenir uniquement des chiffres")
		}
	}
	if baseURL == "" {
		baseURL = "https://conso.boris.sh"
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		prm:        prm,
		httpClient: httpClient,
	}, nil
}

func (c *Client) Consumption(ctx context.Context, start, end time.Time) (Consumption, error) {
	var response struct {
		Grandeur []struct {
			Metier   string `json:"grandeurMetier"`
			Physique string `json:"grandeurPhysique"`
			Unite    string `json:"unite"`
			Points   []struct {
				Date     string `json:"d"`
				Value    string `json:"v"`
				Interval string `json:"p"`
				Nature   string `json:"n"`
			} `json:"points"`
		} `json:"grandeur"`
	}
	if err := c.get(ctx, "courbe_de_charge_consommation", start, end, &response); err != nil {
		return Consumption{}, err
	}
	var result Consumption
	found := false
	for _, series := range response.Grandeur {
		if series.Metier != "CONS" || series.Physique != "PA" {
			continue
		}
		if found || series.Unite != "W" || series.Points == nil {
			return Consumption{}, fmt.Errorf("réponse Conso API invalide: série CONS/PA dupliquée, unité différente de W ou points absents")
		}
		found = true
		for _, point := range series.Points {
			if _, err := time.Parse("2006-01-02 15:04:05", point.Date); err != nil {
				return Consumption{}, fmt.Errorf("horodatage Conso API %q invalide: %w", point.Date, err)
			}
			if point.Interval != "PT15M" && point.Interval != "PT30M" {
				return Consumption{}, fmt.Errorf("intervalle Conso API %q non pris en charge", point.Interval)
			}
			result.IntervalReading = append(result.IntervalReading, Reading{
				Date: point.Date, Value: point.Value, IntervalLength: point.Interval, MeasureType: point.Nature,
			})
		}
	}
	if !found {
		return Consumption{}, fmt.Errorf("réponse Conso API invalide: série CONS/PA absente")
	}
	return result, nil
}

func (c *Client) get(ctx context.Context, dataType string, start, end time.Time, target any) error {
	u, err := url.Parse(c.baseURL + "/api/" + dataType)
	if err != nil {
		return fmt.Errorf("adresse Conso API invalide: %w", err)
	}
	query := u.Query()
	query.Set("pointId", c.prm)
	query.Set("dateDebut", start.Format(time.DateOnly))
	query.Set("dateFin", end.Format(time.DateOnly))
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("création de la requête: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", "github.com/tomgachet/conso-dashboard")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("appel à Conso API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("Conso API a répondu %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("décodage de la réponse: %w", err)
	}
	return nil
}
