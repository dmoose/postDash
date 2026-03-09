package main

import "sync"

// RingBroadcaster is a generic ring buffer with fan-out to subscribers.
// Used by both Hub (events) and LogHub (logs) with independent instances.
type RingBroadcaster[T any] struct {
	mu      sync.RWMutex
	ring    []T
	maxSize int
	clients map[*Subscriber[T]]bool
}

// Subscriber receives items that pass its filter function.
type Subscriber[T any] struct {
	Ch     chan T
	filter func(T) bool
}

// NewRingBroadcaster creates a ring buffer with the given capacity.
func NewRingBroadcaster[T any](maxSize int) *RingBroadcaster[T] {
	return &RingBroadcaster[T]{
		ring:    make([]T, 0, maxSize),
		maxSize: maxSize,
		clients: make(map[*Subscriber[T]]bool),
	}
}

// Append adds an item to the ring and fans it out to matching subscribers.
func (rb *RingBroadcaster[T]) Append(item T) {
	rb.mu.Lock()
	if len(rb.ring) >= rb.maxSize {
		rb.ring = rb.ring[1:]
	}
	rb.ring = append(rb.ring, item)

	clients := make([]*Subscriber[T], 0, len(rb.clients))
	for c := range rb.clients {
		clients = append(clients, c)
	}
	rb.mu.Unlock()

	for _, c := range clients {
		if c.filter(item) {
			select {
			case c.Ch <- item:
			default:
			}
		}
	}
}

// Subscribe creates a subscriber with the given filter.
func (rb *RingBroadcaster[T]) Subscribe(filter func(T) bool) *Subscriber[T] {
	s := &Subscriber[T]{
		Ch:     make(chan T, 64),
		filter: filter,
	}
	rb.mu.Lock()
	rb.clients[s] = true
	rb.mu.Unlock()
	return s
}

// Unsubscribe removes a subscriber and closes its channel.
func (rb *RingBroadcaster[T]) Unsubscribe(s *Subscriber[T]) {
	rb.mu.Lock()
	delete(rb.clients, s)
	rb.mu.Unlock()
	close(s.Ch)
}

// Recent returns the last n items matching the filter, in chronological order.
func (rb *RingBroadcaster[T]) Recent(n int, filter func(T) bool) []T {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var result []T
	for i := len(rb.ring) - 1; i >= 0 && len(result) < n; i-- {
		if filter(rb.ring[i]) {
			result = append(result, rb.ring[i])
		}
	}
	// Reverse to chronological order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// Walk iterates the ring from newest to oldest, calling fn for each item.
// Stops if fn returns false.
func (rb *RingBroadcaster[T]) Walk(fn func(T) bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	for i := len(rb.ring) - 1; i >= 0; i-- {
		if !fn(rb.ring[i]) {
			break
		}
	}
}

// Len returns the number of items currently in the buffer.
func (rb *RingBroadcaster[T]) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return len(rb.ring)
}

// Cap returns the buffer capacity.
func (rb *RingBroadcaster[T]) Cap() int {
	return rb.maxSize
}

// ClientCount returns the number of active subscribers.
func (rb *RingBroadcaster[T]) ClientCount() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return len(rb.clients)
}
