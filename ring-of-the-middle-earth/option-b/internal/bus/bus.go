// Package bus abstracts the event backbone. The core engine talks to this
// interface, NOT to Kafka directly, so `go test`/`go run` work with the
// in-memory implementation while production uses the Confluent client (compiled
// in with `-tags kafka`, see confluent_bus.go).
package bus

import (
	"fmt"
	"sync"
)

// Message is a topic record.
type Message struct {
	Topic string
	Key   string
	Value []byte
}

// Bus is the minimal producer/consumer contract used by the engine and API.
type Bus interface {
	Produce(topic, key string, value []byte) error
	Subscribe(topics ...string) (<-chan Message, error)
	Close() error
}

// MemoryBus is an in-process pub/sub used for local runs and tests. It models the
// 10 Kafka topics as fan-out channels; it is NOT durable (Kafka provides that).
type MemoryBus struct {
	mu     sync.RWMutex
	subs   map[string][]chan Message
	closed bool
}

// NewMemoryBus builds an empty in-memory bus.
func NewMemoryBus() *MemoryBus {
	return &MemoryBus{subs: make(map[string][]chan Message)}
}

// Produce fans a record to every subscriber of the topic (non-blocking; drops to
// a buffered channel — capacity mirrors the Go pipeline buffers in the spec).
func (b *MemoryBus) Produce(topic, key string, value []byte) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return fmt.Errorf("bus closed")
	}
	for _, ch := range b.subs[topic] {
		select {
		case ch <- Message{Topic: topic, Key: key, Value: value}:
		default:
			// subscriber is slow; in a real system Kafka retains the record.
		}
	}
	return nil
}

// Subscribe returns a single merged channel receiving records from all topics.
func (b *MemoryBus) Subscribe(topics ...string) (<-chan Message, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, fmt.Errorf("bus closed")
	}
	ch := make(chan Message, 256)
	for _, t := range topics {
		b.subs[t] = append(b.subs[t], ch)
	}
	return ch, nil
}

// Close stops the bus.
func (b *MemoryBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}
