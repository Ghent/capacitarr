package notifications

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// webhookHTTPClient is a shared HTTP client for outbound webhook requests.
// 10-second timeout keeps notification sends from blocking too long.
var webhookHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// Retry configuration for webhook delivery.
const (
	maxRetries     = 3
	initialBackoff = 1 * time.Second
	backoffFactor  = 2
)

// sendWebhookRequest sends an HTTP POST to the given URL with JSON body,
// retrying on 429 (rate limit) and 5xx (server error) responses with
// exponential backoff. Returns an error if all attempts fail.
func sendWebhookRequest(webhookURL string, body []byte) error {
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("Retrying webhook request",
				"component", "notifications",
				"attempt", attempt+1,
			)
			time.Sleep(backoff)
			backoff *= time.Duration(backoffFactor)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
		if err != nil {
			cancel()
			return fmt.Errorf("create webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := webhookHTTPClient.Do(req) //nolint:gosec // URL is from admin-configured webhook settings
		if err != nil {
			cancel()
			if attempt == maxRetries {
				return fmt.Errorf("webhook request failed after %d attempts: %w", maxRetries+1, err)
			}
			continue
		}

		// Drain and close body to allow connection reuse.
		// LimitReader caps the drain at 1 MiB to prevent a malicious endpoint
		// from keeping the connection open with an infinite response body.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		cancel()

		// Success
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		// Rate limited — respect Retry-After header if present
		if resp.StatusCode == http.StatusTooManyRequests {
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
					backoff = time.Duration(seconds) * time.Second
				}
			}
			if attempt == maxRetries {
				return fmt.Errorf("webhook rate limited (429) after %d attempts", maxRetries+1)
			}
			continue
		}

		// Server error — retry
		if resp.StatusCode >= 500 {
			if attempt == maxRetries {
				return fmt.Errorf("webhook server error (%d) after %d attempts", resp.StatusCode, maxRetries+1)
			}
			continue
		}

		// Client error (4xx except 429) — do not retry
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return fmt.Errorf("webhook failed after %d attempts", maxRetries+1)
}
