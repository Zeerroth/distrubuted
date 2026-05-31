// Package router is the SINGLE enforcement point for information asymmetry
// (spec §30). The Dark Side must never receive the Ring Bearer's true position.
// This is verified by router_test.go run with `go test -race` and by Demo
// Scenario 1.
package router

import (
	"rotr/internal/game"
)

// Kafka topic names (spec §9).
const (
	TopicRingPosition  = "game.ring.position"  // RingBearerMoved   -> Light Side ONLY
	TopicRingDetection = "game.ring.detection" // RingBearerDetected/Spotted -> Dark Side ONLY
	TopicBroadcast     = "game.broadcast"      // WorldStateSnapshot -> both (RB stripped for Dark)
	TopicEventsUnit    = "game.events.unit"
	TopicEventsRegion  = "game.events.region"
	TopicEventsPath    = "game.events.path"
)

// WorldStateSnapshot is the broadcast payload (spec §10). It carries every unit's
// public state. The Ring Bearer's entry has Region=="" already in public state;
// the true region travels ONLY on game.ring.position (Light Side).
type WorldStateSnapshot struct {
	Turn    int                          `json:"turn"`
	Units   map[string]game.UnitState    `json:"units"`
	Regions map[string]game.RegionState  `json:"regions"`
	Paths   map[string]game.PathState    `json:"paths"`
	// RingBearerRegion is the TRUE region. It is populated on the Light-Side copy
	// only; stripRingBearer blanks it (and the ring-bearer unit entry) for Dark.
	RingBearerRegion string `json:"ringBearerRegion,omitempty"`
	// RingLastDetectedRegion/Turn are the LEGITIMATELY revealed-to-Dark detection
	// info (set only when detection fired). NOT stripped — this is the one channel
	// through which the Dark Side is allowed to learn a position. The UI shows it
	// as a "last seen" marker on the Dark board.
	RingLastDetectedRegion string `json:"ringLastDetectedRegion,omitempty"`
	RingLastDetectedTurn   int    `json:"ringLastDetectedTurn,omitempty"`
	// Game-over status, so the UI can show a banner without waiting for the event.
	Over   bool   `json:"over"`
	Winner string `json:"winner,omitempty"`
	Cause  string `json:"cause,omitempty"`
}

// Event is a routable message tagged with its source topic.
type Event struct {
	Topic    string
	Type     string
	Turn     int
	Snapshot *WorldStateSnapshot // for broadcast
	Region   string              // for ring.position / ring.detection payloads
	PathID   string              // for RingBearerSpotted
}

// RingBearerUnitID is the config id of the bearer. It is used ONLY for stripping
// in the I/O boundary (not in game logic) — the one place the spec allows naming
// the bearer, since "strip the bearer's field" is inherently about the bearer.
const RingBearerUnitID = "ring-bearer"

// EventRouter fans events to side-specific channels. Construct with NewEventRouter.
type EventRouter struct {
	LightCh chan Event
	DarkCh  chan Event
}

// NewEventRouter makes a router with buffered side channels.
func NewEventRouter(buffer int) *EventRouter {
	return &EventRouter{
		LightCh: make(chan Event, buffer),
		DarkCh:  make(chan Event, buffer),
	}
}

// Route applies spec §30 exactly. This switch is the ONLY place that decides who
// sees what.
func (r *EventRouter) Route(e Event) {
	switch e.Topic {
	case TopicRingPosition:
		r.LightCh <- e
		// never DarkCh

	case TopicRingDetection:
		r.DarkCh <- e
		// never LightCh

	case TopicBroadcast:
		r.LightCh <- e
		r.DarkCh <- stripRingBearer(e) // RB region blanked for Dark Side

	case TopicEventsUnit, TopicEventsRegion, TopicEventsPath:
		r.LightCh <- e
		r.DarkCh <- e
	}
}

// stripRingBearer returns a deep-enough copy of a broadcast event with the Ring
// Bearer's region removed (spec §30 stripRingBearer). The original (Light) event
// is never mutated.
func stripRingBearer(e Event) Event {
	if e.Snapshot == nil {
		return e
	}
	orig := e.Snapshot
	clone := *orig
	clone.RingBearerRegion = "" // top-level true region blanked

	// copy the units map and blank the ring-bearer entry's region
	clone.Units = make(map[string]game.UnitState, len(orig.Units))
	for id, u := range orig.Units {
		if id == RingBearerUnitID {
			u.Region = ""
		}
		clone.Units[id] = u
	}
	e.Snapshot = &clone
	return e
}
