package integrations

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"time"

	"capacitarr/internal/logger"
)

// sharedHTTPClient is a package-level HTTP client with a 30-second timeout.
//
// IMPORTANT: The CheckRedirect policy preserves the original HTTP method and
// headers across 301/302 redirects. Go's default behaviour silently converts
// DELETE/POST/PUT to GET on 301/302, which causes destructive operations
// (e.g., *arr delete-media calls) to degrade into reads when a reverse proxy
// issues a redirect (trailing-slash, HTTP→HTTPS, path normalisation). The
// redirect response still returns 200 (the GET succeeds), so the caller
// believes the deletion succeeded while the item remains untouched.
var sharedHTTPClient = &http.Client{
	Timeout:       30 * time.Second,
	CheckRedirect: preserveMethodRedirect,
}

// preserveMethodRedirect is an http.Client CheckRedirect policy that
// preserves the original HTTP method and headers across all redirects.
//
// Go's default policy changes POST/PUT/DELETE to GET on 301 (Moved
// Permanently) and 302 (Found) redirects, matching legacy browser behaviour.
// This is correct for browsers but catastrophic for API clients: a DELETE
// silently becomes a GET, the GET succeeds with 200, and the caller reports
// "deleted" while nothing was actually removed. Any user whose *arr instances
// sit behind a reverse proxy that issues 301/302 redirects (trailing-slash
// canonicalisation, HTTP→HTTPS upgrade, path normalisation) is affected.
//
// This policy makes 301/302 behave like 307/308 (method-preserving) and
// copies all headers from the original request so authentication headers
// (X-Api-Key, X-Plex-Token, etc.) survive the redirect.
func preserveMethodRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	original := via[0]
	req.Method = original.Method
	for key, values := range original.Header {
		if _, exists := req.Header[key]; !exists {
			req.Header[key] = values
		}
	}
	return nil
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
	headers := map[string]string{}
	if headerKey != "" {
		headers[headerKey] = headerValue
	}
	return DoAPIRequestWithHeaderMap(url, headers)
}

// DoAPIRequestWithHeaderMap is DoAPIRequest with multiple request headers.
// Used by Jellyfin, which must send both Authorization (Jellyfin 12) and
// the legacy X-Emby-Token header for older servers.
func DoAPIRequestWithHeaderMap(url string, headers map[string]string) ([]byte, error) {
	start := time.Now()
	sanitizedURL := logger.SanitizeURL(url)

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request for %s: %w", sanitizedURL, err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := sharedHTTPClient.Do(req) //nolint:gosec // G704: URL is from admin-configured integration settings
	if err != nil {
		slog.Error("Integration API request failed", "component", "integrations",
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

// DoAPIRequestWithBody creates an HTTP request with the specified method, body,
// and auth header. Used for POST/PUT operations (label management, item updates).
// Accepts non-200 success codes (200, 204) since some APIs return 204 No Content.
func DoAPIRequestWithBody(method, url string, body []byte, headerKey, headerValue string) error {
	start := time.Now()
	sanitizedURL := logger.SanitizeURL(url)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create %s request for %s: %w", method, sanitizedURL, err)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if headerKey != "" {
		req.Header.Set(headerKey, headerValue)
	}

	resp, err := sharedHTTPClient.Do(req) //nolint:gosec // G704: URL is from admin-configured integration settings
	if err != nil {
		slog.Error("Integration API request failed", "component", "integrations",
			"method", method, "url", sanitizedURL, "error", err, "duration", time.Since(start).String())
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	slog.Debug("Integration API response", "component", "integrations",
		"method", method, "url", sanitizedURL, "status", resp.StatusCode, "duration", time.Since(start).String())

	if resp.StatusCode == 401 {
		return fmt.Errorf("unauthorized: invalid API key or token")
	}
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// DoMultipartUpload sends a multipart/form-data POST request with a single file
// field. Used by Plex, which requires multipart uploads to set a poster as the
// active selection (raw body POST only adds the poster to the list without selecting it).
func DoMultipartUpload(url string, imageData []byte, fieldName, fileName string, extraHeaders map[string]string) error {
	start := time.Now()
	sanitizedURL := logger.SanitizeURL(url)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(imageData); err != nil {
		return fmt.Errorf("write form file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, &buf)
	if err != nil {
		return fmt.Errorf("create multipart request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := sharedHTTPClient.Do(req) //nolint:gosec // G704: URL is from admin-configured integration settings
	if err != nil {
		slog.Error("Integration multipart upload failed", "component", "integrations",
			"url", sanitizedURL, "error", err, "duration", time.Since(start).String())
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	slog.Debug("Integration multipart upload response", "component", "integrations",
		"url", sanitizedURL, "status", resp.StatusCode, "duration", time.Since(start).String())

	if resp.StatusCode == 401 {
		return fmt.Errorf("unauthorized: invalid API key or token")
	}
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// DoAPIRequestWithHeaders creates an HTTP request with multiple headers.
// Used when both Content-Type and auth headers are needed (e.g., Jellyfin/Emby
// poster uploads that require Content-Type: image/jpeg AND auth headers).
func DoAPIRequestWithHeaders(method, url string, body []byte, headers map[string]string) error {
	start := time.Now()
	sanitizedURL := logger.SanitizeURL(url)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, url, bodyReader)
	if err != nil {
		return err
	}
	if body != nil && headers["Content-Type"] == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := sharedHTTPClient.Do(req) //nolint:gosec // G704: URL is from admin-configured integration settings
	if err != nil {
		slog.Error("Integration API request failed", "component", "integrations",
			"method", method, "url", sanitizedURL, "error", err, "duration", time.Since(start).String())
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	slog.Debug("Integration API response", "component", "integrations",
		"method", method, "url", sanitizedURL, "status", resp.StatusCode, "duration", time.Since(start).String())

	if resp.StatusCode == 401 {
		return fmt.Errorf("unauthorized: invalid API key or token")
	}
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
