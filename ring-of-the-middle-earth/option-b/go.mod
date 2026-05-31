module rotr

go 1.22

// Core packages (config, game, pipeline, router, bus/memory) depend ONLY on the
// Go standard library, so `go test ./...` and `make test` run with no Docker and
// no Kafka. The real Confluent Kafka client is wired in behind the `kafka` build
// tag (see internal/bus/confluent_bus.go) so it never breaks the offline test run.
