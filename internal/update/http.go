package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// HTTPClient is the injectable transport used for HTTPS downloads and API calls.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

const (
	maxChecksumsBytes  int64 = 1 << 20
	maxManifestBytes   int64 = 1 << 20
	maxTarballBytes    int64 = 512 << 20
	maxDictionaryBytes int64 = 512 << 20
	maxBinaryBytes     int64 = 128 << 20
)

func fetch(ctx context.Context, client HTTPClient, rawURL string, maxBytes int64) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("URL must use https: %s", rawURL)
	}
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", rawURL, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", rawURL, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rawURL, err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%s exceeds the %d byte download limit", parsed.Path, maxBytes)
	}
	return data, nil
}
