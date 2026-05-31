#!/usr/bin/env bash
# Registers every Avro schema in kafka/schemas/ with the Schema Registry.
#
# Subject naming (spec §10): {topicName}-value (TopicNameStrategy). Several event
# TYPES share one topic (e.g. game.events.path carries PathStatusChanged AND
# PathCorrupted). TopicNameStrategy allows only one schema lineage per subject, so
# the "primary" schema is registered under {topic}-value and the secondary types
# are registered under a RecordNameStrategy subject ({namespace}.{Record}) so
# they are still present in the registry (K2). Production could instead union
# them. Requires: curl, jq.
set -euo pipefail
SR="${1:-http://localhost:8085}"
DIR="$(cd "$(dirname "$0")/schemas" && pwd)"

post() { # subject file
  local subject="$1" file="$2"
  local payload
  payload=$(jq -c . "$DIR/$file" | jq -Rs '{schema: .}')
  curl -s -X POST -H "Content-Type: application/vnd.schemaregistry.v1+json" \
    --data "$payload" "$SR/subjects/$subject/versions" >/dev/null \
    && echo "registered $subject  <- $file"
}

# primary schema per topic-value subject
post "game.orders.raw-value"        order-submitted.avsc
post "game.orders.validated-value"  order-validated.avsc
post "game.events.unit-value"       unit-moved.avsc
post "game.events.region-value"     region-control-changed.avsc
post "game.events.path-value"       path-status-changed.avsc
post "game.broadcast-value"         world-state-snapshot.avsc
post "game.ring.position-value"     ring-bearer-moved.avsc
post "game.ring.detection-value"    ring-bearer-detected.avsc
post "game.dlq-value"               dlq-entry.avsc

# secondary event types under RecordNameStrategy subjects
post "rotr.events.BattleResolved"     battle-resolved.avsc
post "rotr.events.PathCorrupted"      path-corrupted.avsc
post "rotr.events.GameOver"           game-over.avsc
post "rotr.events.RingBearerSpotted"  ring-bearer-spotted.avsc

echo "schema registration complete (V2 order-validated registered separately via 'make schema-v2')"
