package integrations

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"capacitarr/internal/logger"
)

// sharedHTTPClient is a package-level HTTP client with a 30-second timeout.
var sharedHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// maxResponseBytes is the maximum response body size we'll read from an
// upstream integration API. This prevents a misconfigured or malicious
// upstream from causing unbounded memory allocation. 64 MiB is generous
// enough for any *arr API response (the largest is Sonarr's full series
// list at ~10-20 MiB for very large libraries).
const maxResponseBytes = 64 << 20 // 64 MiB

// DoAPIRequest creates a GET request to the given URL, sets the specified header,
// executes with the shared client, checks for 401/non-200, and reads the body.
// The response body is limited to maxResponseBytes to prevent denial-of-service
// via oversized upstream responses.
func DoAPIRequest(url, headerKey, headerValue string) ([]byte, error) {
	start := time.Now()
	sanitizedURL := logger.SanitizeURL(url)

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if headerKey != "" {
		req.Header.Set(headerKey, headerValue)
	}

	resp, err := sharedHTTPClient.Do(req) //nolint:gosec // G704: URL is from admin-configured integration settings
	if err != nil {
		slog.Debug("Integration API request failed", "component", "integrations",
			"url", sanitizedURL, "error", err, "duration", time.Since(start).String())
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	slog.Debug("Integration API response", "component", "integrations", //nolint:gosec // G706: sanitizedURL is safe, status/duration are server-side values
		"url", sanitizedURL, "status", resp.StatusCode, "duration", time.Since(start).String())

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("unauthorized: invalid API key or token")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Limit response body to prevent unbounded memory allocation from
	// a misconfigured or malicious upstream service.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Detect HTML responses (indicates reverse proxy login page, wrong URL, etc.)
	if len(body) > 0 && (body[0] == '<' || string(body[:min(len(body), 15)]) == "<!DOCTYPE html>" || string(body[:min(len(body), 6)]) == "<html>") {
		return nil, fmt.Errorf("couldn't connect — got a web page instead of data. Double-check the URL is correct and that the service is reachable from the Capacitarr server. If you're using a reverse proxy, make sure it isn't blocking API requests")
	}

	return body, nil
}
