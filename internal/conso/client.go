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
	Date  string `json:"date"`
	Value string `json:"value"`
}

type DailyConsumption struct {
	UsagePointID    string    `json:"usage_point_id"`
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

func (c *Client) DailyConsumption(ctx context.Context, start, end time.Time) (DailyConsumption, error) {
	u, err := url.Parse(c.baseURL + "/api/daily_consumption")
	if err != nil {
		return DailyConsumption{}, fmt.Errorf("adresse Conso API invalide: %w", err)
	}
	query := u.Query()
	query.Set("prm", c.prm)
	query.Set("start", start.Format(time.DateOnly))
	query.Set("end", end.Format(time.DateOnly))
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return DailyConsumption{}, fmt.Errorf("création de la requête: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", "github.com/tomgachet/conso-dashboard")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DailyConsumption{}, fmt.Errorf("appel à Conso API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return DailyConsumption{}, fmt.Errorf("Conso API a répondu %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var result DailyConsumption
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return DailyConsumption{}, fmt.Errorf("décodage de la réponse: %w", err)
	}
	return result, nil
}
