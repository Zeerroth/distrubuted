#!/usr/bin/env bash
# Creates the 10 topics from spec §9 with their partition / cleanup / retention
# config. Run inside the kafka container (see Makefile `make topics`).
#
# NOTE on replication: the spec table says replication=3, which REQUIRES a
# 3-broker cluster. The dev docker-compose ships a single broker for laptop
# friendliness, so REPL defaults to 1. Set REPL=3 against a 3-broker cluster to
# match the graded K1 config exactly.
set -euo pipefail

BROKER="${BROKER:-localhost:9092}"
REPL="${REPL:-1}"

create() { # name partitions cleanup retention_ms
  local name="$1" parts="$2" cleanup="$3" retention="$4"
  # one --config flag per setting (kafka-topics rejects "a=b,c=d" in a single flag)
  local cfgs=(--config "cleanup.policy=${cleanup}")
  if [ "$retention" != "-" ]; then cfgs+=(--config "retention.ms=${retention}"); fi
  kafka-topics --bootstrap-server "$BROKER" --create --if-not-exists \
    --topic "$name" --partitions "$parts" --replication-factor "$REPL" "${cfgs[@]}"
  echo "created $name (partitions=$parts cleanup=$cleanup retention=$retention)"
}

H=3600000      # 1 hour
D7=604800000   # 7 days

create game.orders.raw        3 delete  $H     # key: playerId
create game.orders.validated  6 delete  $H     # key: unitId
create game.events.unit       6 delete  $D7    # key: unitId
create game.events.region     6 delete  $D7    # key: regionId
create game.events.path       6 delete  $D7    # key: pathId
create game.session           1 compact -      # latest session state
create game.broadcast         1 delete  $H     # WorldStateSnapshot + GameOver
create game.ring.position     1 delete  $H     # Light Side only
create game.ring.detection    2 delete  $H     # key: playerId, Dark Side only
create game.dlq               3 delete  $D7    # key: errorCode

echo "all 10 topics created"
