# Ring of the Middle Earth — Distributed Term Project

**Technology choice: Option B — Go (goroutines + Kafka KTable state stores).**

A browser-based, turn-based, two-player (Human-vs-Human) strategy game backed by a
distributed Go engine and a Kafka event backbone. Light Side moves the Ring Bearer
secretly to Mount Doom; Dark Side hunts it.

> **Academic-integrity note (spec §42).** This repository is built to be
> *understood and defended*, not copy-pasted. Every design decision that the Q&A
> targets is documented inline in the code (search for `spec §`). The combat,
> detection, path and unit logic is **config-driven** — there is no unit-id string
> literal in game logic (`grep -rn '"witch-king"\|"gandalf"\|"sauron"' option-b/internal`
> returns nothing in logic paths). Keep your **LLM-usage log** in
> `ARCHITECTURE.md` honest, and make sure each team member can explain any file.

---

## Layout

```
ring-of-the-middle-earth/
├── docker-compose.yml         kafka (KRaft) + schema-registry + go-1/2/3
├── Makefile                   make up / test / race / topics / schemas
├── config/                    SHARED config (authoritative)
│   ├── units.json  units.conf (HOCON mirror, for reading)
│   └── map.json    map.conf
├── kafka/
│   ├── schemas/               13 Avro schemas + OrderValidated V2
│   ├── streams/README.md      Topology 1 & 2 -> Go mapping
│   ├── create-topics.sh       the 10 topics (spec §9)
│   └── register-schemas.sh
├── option-b/                  the Go engine
│   ├── cmd/server/            main + the 7-case select loop (spec §31)
│   ├── internal/
│   │   ├── config/            config-driven UnitConfig loader
│   │   ├── game/              map, combat, detection, paths, units, validation
│   │   ├── pipeline/          Route-Risk (P1) + Interception (P2)
│   │   ├── router/            EventRouter + WorldStateCache (info hiding)
│   │   ├── engine/            the 13-step turn processor
│   │   ├── bus/               Bus iface: MemoryBus + Kafka (build tag)
│   │   └── api/               HTTP + SSE + delivery-edge hiding
│   └── tests/                 combat / router(-race) / pipeline1 / pipeline2 / detection / state-machine
├── ui/                        vanilla JS + SSE (no frameworks)
└── video-scripts/             demo video scripts for 3 presenters
```

## Run the unit tests (no Docker, no Kafka)

```bash
cd option-b
go test ./...        # all suites
go test -race ./...  # router/cache race checks (B7)
```
or `make test` / `make race` from the repo root.

## Run a single engine locally (in-memory bus, no Kafka)

```bash
make run             # serves the engine AND the UI at http://localhost:8080
```
Then open **http://localhost:8080/** in a browser — the engine serves the UI
same-origin, so it just works (no CORS, no file:// fuss).

## Demo UI — the Command Map (for filming / manual scenario testing)

`ui/` is a vanilla-JS + SVG + SSE single page (no frameworks, spec §37) built to
drive the demo on camera:

- **Live SVG map** of all 22 regions + 37 paths, colour-coded by control
  (green=Free, red=Shadow, grey=Neutral) and path status (open/threatened/blocked/
  temporarily-open/surveilled). Unit tokens sit on their regions.
- **Both view** shows the Light and Dark boards side by side — the Ring Bearer is
  drawn on Light and **hidden on Dark** (a ✦ marks its last detected region). This
  single screen is the Scenario-1 information-hiding shot.
- **One-click Scenarios** (Scenarios tab) play the exact, engine-verified order
  sequences automatically with step-by-step logging: `Information Hiding`,
  `Saruman Corrupts a Path`, `Gandalf Opens a Blocked Path`,
  `Guard Denies a Nazgûl Block`, and `Win Drive`. Click one and narrate.
- **Order builder** (Order tab): pick a side + unit + order; build routes by
  clicking regions on the map or with the canonical-route quick-picks.
- **End Turn ▶**, **Reset ↺** (re-seed between takes), per-board **Analysis** panels.

Endpoints added for the UI/demo: `GET /config/units`, `GET /config/map`,
`POST /game/reset`, plus static UI serving at `/` and permissive CORS.

**Verify the scenarios actually fire** (drives the live API and asserts outcomes):
```powershell
# in one shell:  make run        (or: go run ./cmd/server -config ../config -ui ../ui)
powershell -ExecutionPolicy Bypass -File verify-scenarios.ps1   # expect 15/15 pass
```

## Run the full distributed stack (Kafka + 3 Go instances)

```bash
make up              # builds option-b with -tags kafka, starts kafka + SR + go-1/2/3
                     # creates the 10 topics and registers the Avro schemas
# Light browser -> http://localhost:8081   Dark browser -> http://localhost:8082
make schema-v2       # deploy OrderValidated V2 while V1 consumers run (K3)
make down
```

## Demo scenarios (spec §40)

1. **Information hiding** — open two UI tabs (Light + Dark). Move the Ring Bearer
   and a Nazgul into detection range; only Dark sees `RING_BEARER_DETECTED`, and
   `GET /game/state?side=dark` returns `ring-bearer.region=""`.
2. **Maia dispatch + path mechanics** — the *same* `MAIA_ABILITY` order type opens
   a path for Gandalf and permanently corrupts one for Saruman; a FellowshipGuard
   at an endpoint blocks a Nazgul's block.
3. **Fault tolerance + exactly-once** — `docker stop go-2` mid-turn; the consumer
   group rebalances and go-1/go-3 continue; `GameOver` appears exactly once on
   `game.broadcast`.

## Known spec discrepancies (raised honestly, not silently "fixed")

- §3 says **14 units** and **"DARK SIDE — 7 units"**, but the config block in §3.3
  defines **13** (6 Dark units). We load exactly what `config/units.json` contains
  (13). Because the system is config-driven, the missing 14th unit is a one-line
  config edit and **zero** code change — this is itself the property A2/B1 reward.
- §2.2 header says **35 paths** but the table lists **37** rows (including
  `cirith-ungol-to-mount-doom` and `mordor-to-mount-doom`, which the canonical
  routes require). We include all 37 so all four routes are BFS-discoverable.

These are documented in `ARCHITECTURE.md` §spec-notes.
