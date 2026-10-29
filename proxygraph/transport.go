package proxygraph

import (
	"fmt"
	"net/http"
	"net/url"
)

// proxyTransport builds an http.RoundTripper that routes through proxyURL.
func proxyTransport(proxyURL string) (http.RoundTripper, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("proxygraph: parse proxy url: %w", err)
	}
	return &http.Transport{Proxy: http.ProxyURL(u)}, nil
}
