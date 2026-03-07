package notifications

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSendWebhookRequest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := sendWebhookRequest(server.URL, []byte(`{"test": true}`))
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
}

func TestSendWebhookRequest_ClientError_NoRetry(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	err := sendWebhookRequest(server.URL, []byte(`{"test": true}`))
	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	if callCount.Load() != 1 {
		t.Errorf("expected exactly 1 call (no retries for 4xx), got %d", callCount.Load())
	}
}

func TestSendWebhookRequest_ServerError_Retries(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := callCount.Add(1)
		if count <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := sendWebhookRequest(server.URL, []byte(`{"test": true}`))
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}

	if callCount.Load() != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", callCount.Load())
	}
}

func TestSendWebhookRequest_RateLimit429_Retries(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := callCount.Add(1)
		if count == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := sendWebhookRequest(server.URL, []byte(`{"test": true}`))
	if err != nil {
		t.Fatalf("expected success after 429 retry, got error: %v", err)
	}

	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls (1 rate-limit + 1 success), got %d", callCount.Load())
	}
}

func TestSendWebhookRequest_MaxRetriesExhausted(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := sendWebhookRequest(server.URL, []byte(`{"test": true}`))
	if err == nil {
		t.Fatal("expected error after max retries exhausted")
	}

	// maxRetries = 3, so total calls = 1 initial + 3 retries = 4
	if callCount.Load() != int32(maxRetries+1) {
		t.Errorf("expected %d calls, got %d", maxRetries+1, callCount.Load())
	}
}
