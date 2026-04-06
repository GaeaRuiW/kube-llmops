package sse

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/gin-gonic/gin"
)

type Event struct {
	Type   string      `json:"type"`   // e.g., "model.updated", "finetune.progress", "component.health"
	Object interface{} `json:"object"`
}

type Broker struct {
	mu      sync.RWMutex
	clients map[chan Event]struct{}
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[chan Event]struct{}),
	}
}

func (b *Broker) Subscribe() chan Event {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broker) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *Broker) Broadcast(evt Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- evt:
		default:
			// Drop if client is slow
		}
	}
}

func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// StreamEvents is the Gin handler for SSE connections.
func StreamEvents(broker *Broker) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		ch := broker.Subscribe()
		defer broker.Unsubscribe(ch)

		c.Stream(func(w io.Writer) bool {
			select {
			case evt, ok := <-ch:
				if !ok {
					return false
				}
				data, _ := json.Marshal(evt)
				c.SSEvent("message", string(data))
				return true
			case <-c.Request.Context().Done():
				return false
			}
		})
	}
}
