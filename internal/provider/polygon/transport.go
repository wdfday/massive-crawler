package polygon

import (
	"net/http"
	"time"
)

// baseTransportConfig returns the shared HTTP transport configuration used by Polygon clients.
func baseTransportConfig() *http.Transport {
	return &http.Transport{
		ResponseHeaderTimeout: 10 * time.Minute,
		IdleConnTimeout:       0,
		TLSHandshakeTimeout:   10 * time.Second,
		DisableKeepAlives:     true,
		MaxIdleConns:          0,
		MaxIdleConnsPerHost:   0,
	}
}

// newHTTPClient creates an HTTP client configured for Polygon requests.
func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: baseTransportConfig(),
		Timeout:   10 * time.Minute,
	}
}

// NewCrawler constructs a Crawler with a shared HTTP client.
func NewCrawler() (*Crawler, error) {
	return &Crawler{
		client: newHTTPClient(),
	}, nil
}
