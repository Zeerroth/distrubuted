//go:build !kafka

package main

import (
	"log"

	"rotr/internal/bus"
)

// newBus (default build): in-memory bus, no Kafka required.
func newBus() bus.Bus {
	log.Printf("bus: in-memory (build without -tags kafka)")
	return bus.NewMemoryBus()
}
