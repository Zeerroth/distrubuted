//go:build kafka

package main

import (
	"log"
	"os"

	"rotr/internal/bus"
)

// newBus (kafka build): Confluent-backed bus joined to the shared consumer group
// (spec §26). Brokers + group come from env; all 3 instances share GROUP_ID.
func newBus() bus.Bus {
	brokers := envOr("KAFKA_BROKERS", "localhost:9092")
	group := envOr("GROUP_ID", "rotr-engine")
	b, err := bus.NewKafkaBus(brokers, group)
	if err != nil {
		log.Fatalf("kafka bus: %v", err)
	}
	log.Printf("bus: kafka brokers=%s group=%s", brokers, group)
	return b
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
