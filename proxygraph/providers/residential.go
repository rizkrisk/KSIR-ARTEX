package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Autumn-27/artex/proxygraph"
)

// ResidentialProvider fetches a proxy list from a configured JSON API endpoint.
// The endpoint must return an array of objects with at minimum a "url" field;
// optional fields: "id", "asn".
type ResidentialProvider struct {
	APIURL string
	APIKey string
}

type residentialEntry struct {
	ID  string `json:"id"`
	URL string `json:"url"`
	ASN string `json:"asn"`
}

// Fetch calls the API and returns the proxy list.
func (p *ResidentialProvider) Fetch(ctx context.Context) ([]proxygraph.Proxy, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.APIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("residential provider: build request: %w", err)
	}
	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("residential provider: fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("residential provider: api returned %d", resp.StatusCode)
	}
	var entries []residentialEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("residential provider: decode: %w", err)
	}
	out := make([]proxygraph.Proxy, 0, len(entries))
	for i, e := range entries {
		id := e.ID
		if id == "" {
			id = fmt.Sprintf("res-%d", i)
		}
		out = append(out, proxygraph.Proxy{
			ID:     id,
			URL:    e.URL,
			Type:   "residential",
			ASN:    e.ASN,
			Score:  1.0,
			Active: true,
		})
	}
	return out, nil
}
