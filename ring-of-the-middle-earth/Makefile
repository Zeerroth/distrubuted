# Ring of the Middle Earth — Option B (Go)
SHELL := /bin/bash
SR := http://localhost:8085

.PHONY: help up down test race run topics schemas schema-v2 logs clean tidy

help:
	@echo "make up       - start kafka + schema-registry + 3 go instances"
	@echo "make topics   - create the 10 topics (spec §9)"
	@echo "make schemas  - register all Avro schemas (spec §10)"
	@echo "make schema-v2- register OrderValidated V2 (schema evolution, K3)"
	@echo "make test     - run all unit tests (NO docker / kafka needed)"
	@echo "make race     - run tests with the race detector (router/B7)"
	@echo "make run      - run a single engine locally on :8080 (in-memory bus)"
	@echo "make down     - stop the stack"

# ---- the whole system ----
up:
	docker compose up -d --build
	@echo "waiting for kafka..." && sleep 5
	$(MAKE) topics
	$(MAKE) schemas
	@echo "UI: open ui/index.html  (Light: http://localhost:8081  Dark: http://localhost:8082)"

down:
	docker compose down -v

topics:
	docker compose exec -T kafka bash -lc 'BROKER=kafka:29092 REPL=1 bash -s' < kafka/create-topics.sh

# register every .avsc as {filename-derived subject}-value
schemas:
	@for f in kafka/schemas/order-submitted unit-moved path-status-changed path-corrupted \
	          region-control-changed battle-resolved ring-bearer-moved ring-bearer-detected \
	          ring-bearer-spotted world-state-snapshot game-over dlq-entry; do true; done
	@bash kafka/register-schemas.sh $(SR)

schema-v2:
	@curl -s -X POST -H "Content-Type: application/vnd.schemaregistry.v1+json" \
	  --data "{\"schema\": $$(jq -c . kafka/schemas/order-validated-v2.avsc | jq -Rs .)}" \
	  $(SR)/subjects/game.orders.validated-value/versions | jq .

# ---- offline unit tests (make test must run without Docker) ----
test:
	cd option-b && go test ./...

race:
	cd option-b && go test -race ./...

run:
	cd option-b && go run ./cmd/server -addr :8080 -config ../config

tidy:
	cd option-b && go mod tidy

logs:
	docker compose logs -f go-1 go-2 go-3

clean: down
	docker compose rm -f
