# Kafka Streams Topologies — Option B mapping

The spec (Part 2 §11–§12) describes two stream-processing topologies. In Option B
(Go), the engine consumes `game.orders.validated`, so the same two responsibilities
are implemented **as Go stream processors** that sit between `game.orders.raw` and
the engine. The rules and formulas are byte-for-byte the spec's; only the runtime
(Go consumer group vs. the JVM Kafka Streams library) differs. This is the
Option-A-vs-B architectural divergence the report must discuss.

## Topology 1 — Order Validation
- **Source:** `game.orders.raw`
- **Sinks:** `game.orders.validated` (valid) / `game.dlq` (invalid)
- **KTables consulted:** TurnKTable, UnitKTable, PathKTable, RegionKTable — in Go
  these are the in-memory projections rebuilt from `game.session` + `game.events.*`
  (see `internal/router/cache.go` and the engine state).
- **The 8 rules** are implemented in `option-b/internal/game/validation.go`
  (`game.Validate`). Each rule maps 1:1 to the table in §11 and returns the exact
  error code from §5.4. Covered by the validation path in the engine and exercised
  by submitting one invalid order per rule (demo K4).

| # | Rule | Error code | Code location |
|---|------|-----------|---------------|
| 1 | turn mismatch | WRONG_TURN | `validation.go` rule 1 |
| 2 | not your side | NOT_YOUR_UNIT | rule 2 (reads `cfg.Side`) |
| 3 | next path BLOCKED | PATH_BLOCKED | ASSIGN/REDIRECT branch |
| 4 | path not in graph/route | INVALID_PATH | ASSIGN/REDIRECT branch |
| 5 | unit not at endpoint | UNIT_NOT_ADJACENT | BLOCK/SEARCH branch |
| 6 | target not adjacent/enemy | INVALID_TARGET | ATTACK branch |
| 7 | ability on cooldown | ABILITY_ON_COOLDOWN | MAIA branch |
| 8 | duplicate unit order | DUPLICATE_UNIT_ORDER | `SeenUnits` check |

## Topology 2 — Route Risk Enrichment
- **Source:** `game.orders.validated`, filtered to `ASSIGN_ROUTE` / `REDIRECT_UNIT`.
- The `routeRiskScore` formula is implemented once in
  `option-b/internal/pipeline/routerisk.go` (`RouteRiskScore`) and reused both for
  enrichment and for the `/analysis/routes` endpoint, guaranteeing they agree.
- The enriched record re-emitted to `game.orders.validated` corresponds to the
  **V2** Avro schema (`kafka/schemas/order-validated-v2.avsc`) with the nullable
  `routeRiskScore` field — this is the schema-evolution demo (K3).

## If you instead want the JVM Kafka Streams version
The directory is structured to drop in a `kafka/streams/src/` Kafka Streams
(Java/Scala) project using the same topic names and Avro subjects. The Go
implementation above is the authoritative one for this submission.
