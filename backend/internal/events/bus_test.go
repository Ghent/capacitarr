package events

import (
	"sync"
	"testing"
	"time"
)

// testEvent is a minimal Event implementation for tests.
type testEvent struct {
	typ string
	msg string
}

func (e testEvent) EventType() string    { return e.typ }
func (e testEvent) EventMessage() string { return e.msg }

func TestNewEventBus(t *testing.T) {
	bus := NewEventBus()
	if bus == nil {
		t.Fatal("NewEventBus returned nil")
	}
	if bus.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers, got %d", bus.SubscriberCount())
	}
}

func TestPubSub_SingleSubscriber(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	evt := testEvent{typ: "test", msg: "hello"}
	bus.Publish(evt)

	select {
	case received := <-ch:
		if received.EventType() != "test" {
			t.Fatalf("expected event type 'test', got %q", received.EventType())
		}
		if received.EventMessage() != "hello" {
			t.Fatalf("expected message 'hello', got %q", received.EventMessage())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestPubSub_FanOut(t *testing.T) {
	bus := NewEventBus()
	const numSubs = 5
	channels := make([]chan Event, numSubs)
	for i := range channels {
		channels[i] = bus.Subscribe()
	}

	if bus.SubscriberCount() != numSubs {
		t.Fatalf("expected %d subscribers, got %d", numSubs, bus.SubscriberCount())
	}

	evt := testEvent{typ: "fanout", msg: "broadcast"}
	bus.Publish(evt)

	for i, ch := range channels {
		select {
		case received := <-ch:
			if received.EventType() != "fanout" {
				t.Fatalf("subscriber %d: expected type 'fanout', got %q", i, received.EventType())
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timeout waiting for event", i)
		}
	}

	for _, ch := range channels {
		bus.Unsubscribe(ch)
	}
}

func TestPubSub_BufferOverflow(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Fill the buffer completely
	for i := 0; i < subscriberBufferSize; i++ {
		bus.Publish(testEvent{typ: "fill", msg: "filling"})
	}

	// This should not block — the event is dropped
	bus.Publish(testEvent{typ: "overflow", msg: "dropped"})

	// Drain all events — we should get subscriberBufferSize events (the fill events)
	received := 0
	for {
		select {
		case <-ch:
			received++
		default:
			goto done
		}
	}
done:
	if received != subscriberBufferSize {
		t.Fatalf("expected %d events in buffer, got %d", subscriberBufferSize, received)
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe()

	if bus.SubscriberCount() != 1 {
		t.Fatalf("expected 1 subscriber, got %d", bus.SubscriberCount())
	}

	bus.Unsubscribe(ch)

	if bus.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers after unsubscribe, got %d", bus.SubscriberCount())
	}

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed after unsubscribe")
	}
}

func TestUnsubscribe_Idempotent(t *testing.T) {
	bus := NewEventBus()
	ch := bus.Subscribe()

	bus.Unsubscribe(ch)
	// Second unsubscribe should not panic
	bus.Unsubscribe(ch)

	if bus.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers, got %d", bus.SubscriberCount())
	}
}

func TestClose(t *testing.T) {
	bus := NewEventBus()
	ch1 := bus.Subscribe()
	ch2 := bus.Subscribe()

	bus.Close()

	// All channels should be closed
	if _, ok := <-ch1; ok {
		t.Fatal("expected ch1 to be closed after bus.Close()")
	}
	if _, ok := <-ch2; ok {
		t.Fatal("expected ch2 to be closed after bus.Close()")
	}

	if bus.SubscriberCount() != 0 {
		t.Fatalf("expected 0 subscribers after close, got %d", bus.SubscriberCount())
	}
}

func TestClose_PublishNoop(t *testing.T) {
	bus := NewEventBus()
	bus.Close()

	// Publish after close should not panic
	bus.Publish(testEvent{typ: "after_close", msg: "should be ignored"})
}

func TestClose_SubscribeReturnsClosed(t *testing.T) {
	bus := NewEventBus()
	bus.Close()

	ch := bus.Subscribe()
	_, ok := <-ch
	if ok {
		t.Fatal("expected Subscribe() after Close() to return a closed channel")
	}
}

func TestConcurrencyStress(t *testing.T) {
	bus := NewEventBus()
	const (
		numPublishers  = 10
		numSubscribers = 5
		numEvents      = 100
	)

	channels := make([]chan Event, numSubscribers)
	for i := range channels {
		channels[i] = bus.Subscribe()
	}

	var pubWg sync.WaitGroup
	var conWg sync.WaitGroup

	// Start publishers
	for p := 0; p < numPublishers; p++ {
		pubWg.Add(1)
		go func() {
			defer pubWg.Done()
			for e := 0; e < numEvents; e++ {
				bus.Publish(testEvent{typ: "stress", msg: "concurrent"})
			}
		}()
	}

	// Start consumers
	counts := make([]int, numSubscribers)
	for i, ch := range channels {
		conWg.Add(1)
		go func(idx int, c chan Event) {
			defer conWg.Done()
			for range c {
				counts[idx]++
			}
		}(i, ch)
	}

	// Wait for publishers to finish
	pubWg.Wait()

	// Close bus to close all subscriber channels, ending consumers
	bus.Close()

	// Wait for consumers to drain
	conWg.Wait()

	// Each subscriber should have received events (exact count depends on timing,
	// but should be > 0 since buffer is large enough)
	for i, count := range counts {
		if count == 0 {
			t.Errorf("subscriber %d received 0 events", i)
		}
	}
}
