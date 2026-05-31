package api

import (
	"encoding/json"
	"sync"

	"rotr/internal/bus"
	"rotr/internal/router"
)

// Hub consumes the Kafka topics and fans each record to side-specific SSE
// subscribers, applying the §30 information-hiding rules at the delivery edge.
// It is the EventRouter from spec §30 wired to live SSE connections.
type Hub struct {
	mu    sync.RWMutex
	light map[chan []byte]struct{}
	dark  map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{light: map[chan []byte]struct{}{}, dark: map[chan []byte]struct{}{}}
}

// Subscribe registers an SSE client for a side and returns its channel + an
// unsubscribe func.
func (h *Hub) Subscribe(side string) (chan []byte, func()) {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	if side == "dark" {
		h.dark[ch] = struct{}{}
	} else {
		h.light[ch] = struct{}{}
	}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.light, ch)
		delete(h.dark, ch)
		h.mu.Unlock()
		close(ch)
	}
}

func (h *Hub) toLight(b []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.light {
		select {
		case ch <- b:
		default:
		}
	}
}

func (h *Hub) toDark(b []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.dark {
		select {
		case ch <- b:
		default:
		}
	}
}

// Run consumes the bus and applies the routing switch (spec §30). This is the
// single enforcement point: ring.position never reaches Dark; broadcast is
// stripped for Dark.
func (h *Hub) Run(b bus.Bus) error {
	ch, err := b.Subscribe(
		router.TopicRingPosition, router.TopicRingDetection, router.TopicBroadcast,
		router.TopicEventsUnit, router.TopicEventsRegion, router.TopicEventsPath,
	)
	if err != nil {
		return err
	}
	go func() {
		for msg := range ch {
			switch msg.Topic {
			case router.TopicRingPosition:
				h.toLight(msg.Value) // Light Side ONLY

			case router.TopicRingDetection:
				h.toDark(msg.Value) // Dark Side ONLY

			case router.TopicBroadcast:
				h.toLight(msg.Value)
				h.toDark(stripBroadcast(msg.Value)) // RB region stripped for Dark

			default: // unit / region / path events -> both
				h.toLight(msg.Value)
				h.toDark(msg.Value)
			}
		}
	}()
	return nil
}

// stripBroadcast blanks the Ring Bearer's region in a serialized broadcast for
// the Dark Side (spec §30 stripRingBearer).
func stripBroadcast(raw []byte) []byte {
	var env struct {
		Type     string                     `json:"type"`
		Snapshot router.WorldStateSnapshot  `json:"snapshot"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return raw
	}
	env.Snapshot.RingBearerRegion = ""
	if u, ok := env.Snapshot.Units[router.RingBearerUnitID]; ok {
		u.Region = ""
		env.Snapshot.Units[router.RingBearerUnitID] = u
	}
	out, err := json.Marshal(env)
	if err != nil {
		return raw
	}
	return out
}
