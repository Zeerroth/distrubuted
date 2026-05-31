# Architecture Document — Ring of the Middle Earth (Option B, Go)

> Submit as PDF in the repo root (spec §39). Sections 1–4 are written; **§5
> Reflection** (≥300 words) and **Appendix A: LLM usage log** must be completed by
> the team in your own words before submission — they are graded for honesty.

---

## 1. System diagram

```
 Browser A (Light) ── HTTP/SSE ─┐                        ┌─ HTTP/SSE ── Browser B (Dark)
   POST /order                  │                        │                 POST /order
   GET  /events?side=light      │                        │                 GET  /events?side=dark
   GET  /analysis/routes        ▼                        ▼                 GET  /analysis/intercept
                        ┌────────────────────── Go HTTP/SSE layer (api) ──────────────────────┐
                        │  produces -> game.orders.raw      Hub consumes broadcast/ring/events │
                        │  validate+apply -> engine         applies §30 hiding at delivery edge │
                        └───────────────────────────────┬─────────────────────────────────────┘
                                                         │ Bus interface
                          ┌──────────────────────────────▼──────────────────────────────┐
                          │                     Kafka (10 topics)                         │
                          │  orders.raw  orders.validated  events.{unit,region,path}      │
                          │  session(compact)  broadcast  ring.position  ring.detection   │
                          │  dlq        + Schema Registry (Avro)                           │
                          └──────────────────────────────┬──────────────────────────────┘
                                                         │ consumer group "rotr-engine"
                        ┌────────────────────────────────┼────────────────────────────────┐
                        │ go-1            go-2            go-3   (stateless; KTable views)  │
                        │  TurnProcessor (13-step) · Pipelines P1/P2 · EventRouter         │
                        └─────────────────────────────────────────────────────────────────┘
```

State lives in Kafka (KTable projections); the 3 Go instances are interchangeable
members of one consumer group. Killing one triggers a rebalance (Scenario 3).

## 2. Goroutine map (spec §28/§39)

| Goroutine | Inputs | Outputs | Buffer | Termination |
|---|---|---|---|---|
| Kafka consumers (per topic) | Kafka | `eventCh` | 256 | bus.Close / ctx |
| Hub (EventRouter) | bus merged ch | `light`/`dark` SSE chans | 64/conn | channel close |
| CacheManager (WorldStateCache) | snapshots | value copies | — | process exit |
| TurnProcessor (engine) | validated orders + timer | `game.events.*`, broadcast | — | ctx cancel |
| Pipeline 1 (Route Risk) | dispatcher | 4 workers → aggregator | 20 | ctx/2s timeout, WaitGroup |
| Pipeline 2 (Interception) | dispatcher | 4 workers → aggregator | 30 | ctx/2s timeout, WaitGroup |
| SSE writer (per player) | side channel | HTTP response | 64 | request ctx Done |
| main select loop | 7 cases (spec §31) | — | — | signal / ctx |

No goroutine leaks: every worker pool joins on a `sync.WaitGroup`, every pipeline
honours `context.Context` (or-done), SSE goroutines exit on `r.Context().Done()`.
Verify with `pprof` after 10 turns (B9).

## 3. Kafka topic diagram (spec §9)

| Topic | Key | Part. | Cleanup | Producer | Consumer |
|---|---|---|---|---|---|
| game.orders.raw | playerId | 3 | delete 1h | api `/order` | validator |
| game.orders.validated | unitId | 6 | delete 1h | validator / P2 enrich | engine |
| game.events.unit | unitId | 6 | delete 7d | engine | UnitKTable, SSE |
| game.events.region | regionId | 6 | delete 7d | engine | RegionKTable, SSE |
| game.events.path | pathId | 6 | delete 7d | engine | PathKTable, SSE |
| game.session | — | 1 | **compact** | engine | all (turn/session recovery) |
| game.broadcast | — | 1 | delete 1h | engine | both SSE (Dark stripped) |
| game.ring.position | — | 1 | delete 1h | engine | **Light SSE only** |
| game.ring.detection | playerId | 2 | delete 1h | engine | **Dark SSE only** |
| game.dlq | errorCode | 3 | delete 7d | validator | ops |

**Partition-key rationale:** keying `events.unit` by `unitId` (and `region`/`path`
likewise) keeps all updates to one entity on one partition → per-entity ordering,
the KTable analog of Akka's single-writer-per-shard. `orders.raw` by `playerId`
fairly interleaves a player's orders; `ring.*` are single-partition because they
are a global, low-volume secret channel whose ordering matters.

**Recovery (Q&A 8):** `game.session` is log-compacted, so a restarting instance
reads the latest turn/session record immediately; `game.broadcast` (delete, 1h)
is *not* the source of truth for recovery — the engine rebuilds world state by
replaying the compacted `game.session` plus its assigned `game.events.*`
partitions, then resumes.

## 4. Paradigm justification (spec §39.4)

**Why Go + Kafka suits this problem.** The game is a stream of small, keyed events
(orders → validated → world events) with a fixed 13-step turn pipeline. Go's
goroutines + channels express the fan-out/fan-in analysis pipelines directly, and
Kafka consumer groups give fault tolerance *for free*: the application tier is
stateless and any instance can serve any request because authoritative state lives
in the broker. Information hiding collapses to **one** enforcement point
(`EventRouter`, §30), which is easy to audit and race-test.

**What is genuinely harder than with Akka.** (1) *Single-writer consistency.* Akka
cluster sharding guarantees exactly one `UnitActor` per unit; in Go we must lean on
Kafka partitioning (key = unitId) to get the same single-writer property, and a
cross-entity transaction (combat touching several units/regions) is not a built-in
primitive — we serialize it inside the TurnProcessor. (2) *Stateful recovery.* Akka
Persistence replays an actor's own journal; in Go we hand-roll KTable rebuilds from
compacted topics and must reason about partition assignment after a rebalance.

**How Akka would solve our two hardest parts.** (a) The combat transaction across
units/regions becomes a request/response choreography between `UnitActor`s and a
`RegionActor`, with the `WorldStateActor` singleton sequencing the turn — no manual
locking. (b) Our manual KTable recovery becomes Akka Persistence + cluster shard
rebalancing: the shard coordinator re-homes `UnitActor`s on surviving nodes and
each actor recovers from its event journal/snapshot automatically.

## 5. Reflection (≥300 words) — **TEAM TO COMPLETE**

> Write in your own words. Prompts: What was harder than expected (the 14-vs-13
> unit discrepancy? leadership stacking in combat? enforcing `DarkView=""` under
> `-race`? KTable recovery after rebalance?). What would you design differently
> (e.g. add an explicit `amplifiesDetection` config flag instead of identifying
> Sauron by `Maia && Shadow && Indestructible`; model combat as a saga)? …

---

## Appendix A — LLM usage log (spec §42) — **TEAM TO COMPLETE, graded for honesty**

For each interaction record: the prompt, what you used, what you changed/rejected.

| # | Prompt (summary) | What we used | What we changed / rejected |
|---|---|---|---|
| 1 | | | |
| 2 | | | |

## Appendix B — Spec notes / discrepancies

- **Units:** §3 says 14 (Dark = 7) but §3.3 defines 13 (Dark = 6). We load exactly
  what `config/units.json` holds; adding the 14th is a config-only change.
- **Paths:** §2.2 header says 35; the table lists 37. We include all 37 so all four
  canonical routes are BFS-discoverable (Route 1/4 need `cirith-ungol-to-mount-doom`,
  Route 3 needs `mordor-to-mount-doom`).
- **Sauron identity:** identified by config flags (`Maia && Side==SHADOW &&
  Indestructible`) rather than id; a cleaner design adds an `amplifiesDetection`
  bool — noted for the reflection.
- **Detection distance** = graph hops (BFS); **route/intercept turns** = summed
  path cost (Dijkstra). Both live in `internal/game/map.go`.
