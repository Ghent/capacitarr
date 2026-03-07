package events

import (
	"strings"
	"testing"
	"time"
)

func TestSSEBroadcaster_StartStop(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	broadcaster := NewSSEBroadcaster(bus)
	broadcaster.Start()

	if broadcaster.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", broadcaster.ClientCount())
	}

	broadcaster.Stop()
}

func TestSSEBroadcaster_BroadcastFormatsSSE(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	broadcaster := NewSSEBroadcaster(bus)
	broadcaster.Start()

	// Manually register a client to capture the broadcast
	client := &sseClient{
		events: make(chan []byte, 10),
		done:   make(chan struct{}),
	}
	broadcaster.mu.Lock()
	broadcaster.clients[client] = struct{}{}
	broadcaster.mu.Unlock()

	// Publish an event
	bus.Publish(EngineStartEvent{ExecutionMode: "dry-run"})

	// Wait for broadcast
	select {
	case msg := <-client.events:
		sseMsg := string(msg)
		// Verify SSE format: id, event, data fields
		if !strings.Contains(sseMsg, "id: ") {
			t.Error("SSE message missing 'id' field")
		}
		if !strings.Contains(sseMsg, "event: engine_start") {
			t.Error("SSE message missing 'event: engine_start' field")
		}
		if !strings.Contains(sseMsg, "data: ") {
			t.Error("SSE message missing 'data' field")
		}
		// Verify JSON payload contains executionMode
		if !strings.Contains(sseMsg, `"executionMode":"dry-run"`) {
			t.Errorf("expected executionMode in payload, got: %s", sseMsg)
		}
		// Verify message field is injected
		if !strings.Contains(sseMsg, `"message":"`) {
			t.Errorf("expected message field in payload, got: %s", sseMsg)
		}
		// Verify double newline termination
		if !strings.HasSuffix(sseMsg, "\n\n") {
			t.Error("SSE message should end with double newline")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for SSE broadcast")
	}

	broadcaster.Stop()
}

func TestSSEBroadcaster_FanOutToMultipleClients(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	broadcaster := NewSSEBroadcaster(bus)
	broadcaster.Start()

	clients := make([]*sseClient, 3)
	for i := range clients {
		clients[i] = &sseClient{
			events: make(chan []byte, 10),
			done:   make(chan struct{}),
		}
		broadcaster.mu.Lock()
		broadcaster.clients[clients[i]] = struct{}{}
		broadcaster.mu.Unlock()
	}

	// Publish
	bus.Publish(LoginEvent{Username: "admin"})

	// All clients should receive
	for i, client := range clients {
		select {
		case msg := <-client.events:
			if !strings.Contains(string(msg), "event: login") {
				t.Errorf("client %d received wrong event", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("client %d: timeout waiting for event", i)
		}
	}

	broadcaster.Stop()
}

func TestSSEBroadcaster_ClientBufferFull(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	broadcaster := NewSSEBroadcaster(bus)
	broadcaster.Start()

	// Client with tiny buffer
	client := &sseClient{
		events: make(chan []byte, 1),
		done:   make(chan struct{}),
	}
	broadcaster.mu.Lock()
	broadcaster.clients[client] = struct{}{}
	broadcaster.mu.Unlock()

	// Fill client buffer
	bus.Publish(EngineStartEvent{ExecutionMode: "dry-run"})
	time.Sleep(50 * time.Millisecond)

	// This should not block even though client buffer is full
	bus.Publish(EngineCompleteEvent{Evaluated: 10, Flagged: 2})
	time.Sleep(50 * time.Millisecond)

	// Drain what we can
	received := 0
	for {
		select {
		case <-client.events:
			received++
		default:
			goto done
		}
	}
done:
	if received < 1 {
		t.Error("expected at least 1 event received")
	}

	broadcaster.Stop()
}

func TestSSEBroadcaster_RingBufferStoresEvents(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	broadcaster := NewSSEBroadcaster(bus)
	broadcaster.Start()

	// Publish several events (no clients connected)
	for i := 0; i < 5; i++ {
		bus.Publish(EngineStartEvent{ExecutionMode: "dry-run"})
	}

	time.Sleep(100 * time.Millisecond)

	// Check ring buffer has entries
	broadcaster.ringMu.RLock()
	hasEntries := false
	for i := 0; i < ringBufferSize; i++ {
		if broadcaster.ring[i].payload != nil {
			hasEntries = true
			break
		}
	}
	broadcaster.ringMu.RUnlock()

	if !hasEntries {
		t.Error("expected ring buffer to contain entries")
	}

	broadcaster.Stop()
}

func TestSSEBroadcaster_ReplayMissedEvents(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	broadcaster := NewSSEBroadcaster(bus)
	broadcaster.Start()

	// No clients connected — events go into ring buffer only
	bus.Publish(LoginEvent{Username: "user1"})
	bus.Publish(EngineStartEvent{ExecutionMode: "approval"})
	bus.Publish(EngineCompleteEvent{Evaluated: 50, Flagged: 5})

	time.Sleep(100 * time.Millisecond)

	// Now connect a client and replay from ID 1
	client := &sseClient{
		events: make(chan []byte, 64),
		done:   make(chan struct{}),
	}
	broadcaster.mu.Lock()
	broadcaster.clients[client] = struct{}{}
	broadcaster.mu.Unlock()

	// Replay events after ID 1
	broadcaster.replay(client, "1")

	// Should receive events with ID > 1
	received := 0
	for {
		select {
		case <-client.events:
			received++
		default:
			goto done
		}
	}
done:
	if received != 2 {
		t.Errorf("expected 2 replayed events (IDs 2 and 3), got %d", received)
	}

	broadcaster.Stop()
}

func TestSSEBroadcaster_ReplayInvalidLastEventID(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	broadcaster := NewSSEBroadcaster(bus)
	broadcaster.Start()

	client := &sseClient{
		events: make(chan []byte, 10),
		done:   make(chan struct{}),
	}

	// Invalid Last-Event-ID should be silently ignored
	broadcaster.replay(client, "not-a-number")

	select {
	case <-client.events:
		t.Error("should not receive events with invalid Last-Event-ID")
	default:
		// Expected: no events
	}

	broadcaster.Stop()
}

func TestSSEBroadcaster_ClientCount(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	broadcaster := NewSSEBroadcaster(bus)
	broadcaster.Start()

	if broadcaster.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", broadcaster.ClientCount())
	}

	client1 := &sseClient{events: make(chan []byte, 10), done: make(chan struct{})}
	client2 := &sseClient{events: make(chan []byte, 10), done: make(chan struct{})}

	broadcaster.mu.Lock()
	broadcaster.clients[client1] = struct{}{}
	broadcaster.clients[client2] = struct{}{}
	broadcaster.mu.Unlock()

	if broadcaster.ClientCount() != 2 {
		t.Errorf("expected 2 clients, got %d", broadcaster.ClientCount())
	}

	broadcaster.removeClient(client1)
	if broadcaster.ClientCount() != 1 {
		t.Errorf("expected 1 client after removal, got %d", broadcaster.ClientCount())
	}

	broadcaster.Stop()
}

func TestSSEBroadcaster_MessageContainsHumanReadableMessage(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	broadcaster := NewSSEBroadcaster(bus)
	broadcaster.Start()

	client := &sseClient{
		events: make(chan []byte, 10),
		done:   make(chan struct{}),
	}
	broadcaster.mu.Lock()
	broadcaster.clients[client] = struct{}{}
	broadcaster.mu.Unlock()

	bus.Publish(DeletionSuccessEvent{
		MediaName: "Firefly",
		MediaType: "show",
		SizeBytes: 5069636198,
	})

	select {
	case msg := <-client.events:
		sseMsg := string(msg)
		// The message field should contain the human-readable EventMessage()
		if !strings.Contains(sseMsg, "Firefly") {
			t.Errorf("expected 'Firefly' in SSE payload, got: %s", sseMsg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for SSE message")
	}

	broadcaster.Stop()
}

func TestSSEBroadcaster_IncrementingEventIDs(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	broadcaster := NewSSEBroadcaster(bus)
	broadcaster.Start()

	client := &sseClient{
		events: make(chan []byte, 10),
		done:   make(chan struct{}),
	}
	broadcaster.mu.Lock()
	broadcaster.clients[client] = struct{}{}
	broadcaster.mu.Unlock()

	bus.Publish(EngineStartEvent{ExecutionMode: "dry-run"})
	bus.Publish(EngineCompleteEvent{Evaluated: 10, Flagged: 2})

	time.Sleep(100 * time.Millisecond)

	msg1 := string(<-client.events)
	msg2 := string(<-client.events)

	if !strings.Contains(msg1, "id: 1\n") {
		t.Errorf("expected first event to have id: 1, got: %s", msg1)
	}
	if !strings.Contains(msg2, "id: 2\n") {
		t.Errorf("expected second event to have id: 2, got: %s", msg2)
	}

	broadcaster.Stop()
}
