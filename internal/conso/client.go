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
	var result Consumption
	if err := c.get(ctx, "consumption_load_curve", start, end, &result); err != nil {
		return Consumption{}, err
	}
	return result, nil
}

func (c *Client) get(ctx context.Context, dataType string, start, end time.Time, target any) error {
	u, err := url.Parse(c.baseURL + "/api/" + dataType)
	if err != nil {
		return fmt.Errorf("adresse Conso API invalide: %w", err)
	}
	query := u.Query()
	query.Set("prm", c.prm)
	query.Set("start", start.Format(time.DateOnly))
	query.Set("end", end.Format(time.DateOnly))
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
