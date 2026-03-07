package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSetAndGet(t *testing.T) {
	c := New(1 * time.Minute)
	defer c.Close()

	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("Expected key1 to be found")
	}
	if val != "value1" {
		t.Errorf("Expected value1, got %v", val)
	}
}

func TestCacheMiss(t *testing.T) {
	c := New(1 * time.Minute)
	defer c.Close()

	val, ok := c.Get("nonexistent")
	if ok {
		t.Error("Expected cache miss for nonexistent key")
	}
	if val != nil {
		t.Errorf("Expected nil value for cache miss, got %v", val)
	}
}

func TestOverwriteExistingKey(t *testing.T) {
	c := New(1 * time.Minute)
	defer c.Close()

	c.Set("key1", "first")
	c.Set("key1", "second")

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("Expected key1 to be found after overwrite")
	}
	if val != "second" {
		t.Errorf("Expected 'second' after overwrite, got %v", val)
	}
}

func TestTTLExpiry(t *testing.T) {
	c := New(50 * time.Millisecond)
	defer c.Close()

	c.Set("ephemeral", "data")

	// Should be present immediately
	val, ok := c.Get("ephemeral")
	if !ok {
		t.Fatal("Expected key to be present before TTL expiry")
	}
	if val != "data" {
		t.Errorf("Expected 'data', got %v", val)
	}

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("ephemeral")
	if ok {
		t.Error("Expected cache miss after TTL expiry")
	}
}

func TestInvalidateKey(t *testing.T) {
	c := New(1 * time.Minute)
	defer c.Close()

	c.Set("key1", "value1")
	c.Set("key2", "value2")

	c.Invalidate("key1")

	_, ok := c.Get("key1")
	if ok {
		t.Error("Expected key1 to be invalidated")
	}

	// key2 should still exist
	val, ok := c.Get("key2")
	if !ok {
		t.Error("Expected key2 to still be present")
	}
	if val != "value2" {
		t.Errorf("Expected 'value2', got %v", val)
	}
}

func TestInvalidatePrefix(t *testing.T) {
	c := New(1 * time.Minute)
	defer c.Close()

	c.Set("1:quality", "HD")
	c.Set("1:tags", "anime")
	c.Set("2:quality", "4K")
	c.Set("other", "data")

	c.InvalidatePrefix("1:")

	_, ok := c.Get("1:quality")
	if ok {
		t.Error("Expected 1:quality to be invalidated")
	}
	_, ok = c.Get("1:tags")
	if ok {
		t.Error("Expected 1:tags to be invalidated")
	}

	// Keys with different prefix should survive
	val, ok := c.Get("2:quality")
	if !ok {
		t.Error("Expected 2:quality to still be present")
	}
	if val != "4K" {
		t.Errorf("Expected '4K', got %v", val)
	}

	val, ok = c.Get("other")
	if !ok {
		t.Error("Expected 'other' to still be present")
	}
	if val != "data" {
		t.Errorf("Expected 'data', got %v", val)
	}
}

func TestInvalidateAll(t *testing.T) {
	c := New(1 * time.Minute)
	defer c.Close()

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	c.InvalidateAll()

	for _, key := range []string{"a", "b", "c"} {
		_, ok := c.Get(key)
		if ok {
			t.Errorf("Expected key %q to be invalidated after InvalidateAll", key)
		}
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	c := New(1 * time.Minute)
	defer c.Close()

	const goroutines = 50
	const iterations = 100
	var wg sync.WaitGroup

	// Concurrent writes
	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				key := fmt.Sprintf("key-%d-%d", id, j)
				c.Set(key, j)
			}
		}(i)
	}

	// Concurrent reads alongside writes
	for i := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				key := fmt.Sprintf("key-%d-%d", id, j)
				c.Get(key) //nolint:errcheck // deliberate: testing concurrent access
			}
		}(i)
	}

	wg.Wait()

	// Verify at least some keys were written successfully
	val, ok := c.Get(fmt.Sprintf("key-%d-%d", 0, 0))
	if !ok {
		t.Error("Expected at least key-0-0 to exist after concurrent writes")
	}
	if val != 0 {
		t.Errorf("Expected value 0, got %v", val)
	}
}

func TestDifferentValueTypes(t *testing.T) {
	c := New(1 * time.Minute)
	defer c.Close()

	c.Set("int", 42)
	c.Set("string", "hello")
	c.Set("float", 3.14)
	c.Set("slice", []string{"a", "b"})
	c.Set("nil", nil)

	tests := []struct {
		key      string
		expected any
	}{
		{"int", 42},
		{"string", "hello"},
		{"float", 3.14},
		{"nil", nil},
	}

	for _, tc := range tests {
		val, ok := c.Get(tc.key)
		if !ok {
			t.Errorf("Expected key %q to be found", tc.key)
			continue
		}
		if val != tc.expected {
			t.Errorf("Key %q: expected %v, got %v", tc.key, tc.expected, val)
		}
	}

	// Slice needs separate check (not comparable with ==)
	val, ok := c.Get("slice")
	if !ok {
		t.Fatal("Expected key 'slice' to be found")
	}
	sl, ok := val.([]string)
	if !ok {
		t.Fatalf("Expected []string, got %T", val)
	}
	if len(sl) != 2 || sl[0] != "a" || sl[1] != "b" {
		t.Errorf("Expected [a b], got %v", sl)
	}
}
