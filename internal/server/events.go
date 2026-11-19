package server

import (
	"sync"

	"cloud-phone/internal/model"
)

// Hub fans service changes out to SSE subscribers. A slow subscriber loses
// events rather than blocking the service; clients resync from a snapshot.
type Hub struct {
	mu     sync.Mutex
	next   int64
	subs   map[int64]chan model.Change
	closed bool
}

func NewHub() *Hub {
	return &Hub{subs: map[int64]chan model.Change{}}
}

// Publish delivers a change to every subscriber without blocking.
func (h *Hub) Publish(c model.Change) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	for _, ch := range h.subs {
		select {
		case ch <- c:
		default:
		}
	}
}

// Subscribe returns a channel and a function that removes the subscriber.
// The channel is closed when the hub closes or the subscription ends.
func (h *Hub) Subscribe() (<-chan model.Change, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next++
	id := h.next
	ch := make(chan model.Change, 16)
	if h.closed {
		close(ch)
		return ch, func() {}
	}
	h.subs[id] = ch
	var once sync.Once
	unsub := func() {
		once.Do(func() {
			h.mu.Lock()
			if sub, ok := h.subs[id]; ok {
				delete(h.subs, id)
				close(sub)
			}
			h.mu.Unlock()
		})
	}
	return ch, unsub
}

// Close ends every stream. Safe to call more than once.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for id, ch := range h.subs {
		delete(h.subs, id)
		close(ch)
	}
}
