package sse

import (
	"sync"
	"testing"
	"time"
)

func TestBroker_SubscribeUnsubscribe(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	if b.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", b.ClientCount())
	}
	b.Unsubscribe(ch)
	if b.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", b.ClientCount())
	}
}

func TestBroker_Broadcast(t *testing.T) {
	b := NewBroker()
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	evt := Event{Type: "test", Object: "hello"}
	b.Broadcast(evt)

	select {
	case got := <-ch1:
		if got.Type != "test" {
			t.Errorf("ch1: expected 'test', got '%s'", got.Type)
		}
	case <-time.After(time.Second):
		t.Error("ch1: timeout")
	}

	select {
	case got := <-ch2:
		if got.Type != "test" {
			t.Errorf("ch2: expected 'test', got '%s'", got.Type)
		}
	case <-time.After(time.Second):
		t.Error("ch2: timeout")
	}

	b.Unsubscribe(ch1)
	b.Unsubscribe(ch2)
}

func TestBroker_ConcurrentBroadcast(t *testing.T) {
	b := NewBroker()
	var wg sync.WaitGroup

	// Start 10 subscribers
	channels := make([]chan Event, 10)
	for i := 0; i < 10; i++ {
		channels[i] = b.Subscribe()
	}

	// Broadcast 100 events concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			b.Broadcast(Event{Type: "test", Object: n})
		}(i)
	}
	wg.Wait()

	// Verify at least some events received
	for i, ch := range channels {
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
		if received == 0 {
			t.Errorf("channel %d received 0 events", i)
		}
		b.Unsubscribe(ch)
	}
}

func TestBroker_SlowClientDropped(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()

	// Fill the buffer (64 capacity)
	for i := 0; i < 100; i++ {
		b.Broadcast(Event{Type: "fill", Object: i})
	}

	// Channel should have 64 events (buffer), rest dropped
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 64 {
		t.Errorf("expected 64 buffered events, got %d", count)
	}
	b.Unsubscribe(ch)
}
