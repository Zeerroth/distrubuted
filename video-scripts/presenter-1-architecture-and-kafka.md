# Presenter A — Intro · Architecture · Config-Driven Design · Kafka

**Target length: 5:00.** Covers rubric **A2/B1** (no hardcoded unit ids), **K1**
(10 topics), **K2** (schemas registered), **K3** (schema evolution), **K4**
(8 validation rules → error codes), **K5** (routeRiskScore enrichment).

On-screen plan: terminal + editor + Schema Registry HTTP calls. No gameplay yet.

> **⚠ Windows / PowerShell note.** The `bash` commands below (grep, curl, jq,
> `| head`, `make …`) do **not** run in PowerShell. On this machine use **PowerShell**
> and the `demo.ps1` helper — every command has a `.\demo.ps1 <verb>` form shown
> beside it. Open PowerShell in the repo root:
> ```powershell
> cd C:\Users\ekrem\Desktop\Ders\distrubuted\ödev\ring-of-the-middle-earth
> ```
> **Order for this part:** `.\demo.ps1 check` (no Docker) → start the Kafka stack
> `.\demo.ps1 up` → `.\demo.ps1 topics` → `.\demo.ps1 schemas` → then the
> `describe` / `subjects` / `evolve` verbs as you reach each beat. If you’d rather
> use bash, open **Git Bash** (installed at `C:\Program Files\Git\bin\bash.exe`),
> but `make`/`jq` still aren’t installed — prefer `demo.ps1`.

---

### [0:00–0:35] Cold open / intro

> *(On camera or voiceover over the title slide.)*
> "Hi — we're Team _____, and this is **Ring of the Middle Earth**, our distributed
> term project. It's a two-player, turn-based strategy game: the Light Side smuggles
> the Ring Bearer to Mount Doom while the Dark Side hunts it. We chose **Option B —
> Go with a Kafka event backbone**. I'll cover the architecture and the Kafka layer;
> then Presenter B plays a live game and shows information hiding; then Presenter C
> covers the Maia mechanics and fault tolerance. Let's start with the big picture."

**Action:** show the system diagram slide (from `ARCHITECTURE.md` §1).

### [0:35–1:25] Architecture overview

> "Two browsers talk HTTP and Server-Sent-Events to a **stateless Go layer**. Every
> order is published to Kafka's `game.orders.raw`. It's validated, and valid orders
> flow to the engine, which runs a fixed **13-step turn** and emits world events
> back through Kafka to the browsers. The key idea of Option B: **the application
> tier is stateless, the broker holds the state.** We run **three** Go instances in
> one Kafka **consumer group** — any instance can serve any request, and if one
> dies, Kafka rebalances its partitions to the others. Presenter C will kill one
> live."

**Action:** trace the arrows on the diagram: browser → orders.raw → validate →
engine → events → broadcast → browser.

### [1:25–2:30] Config-driven design — **A2 / B1** (the anti-generation check)

> "The assignment's hardest constraint: **all 14 unit types share one
> implementation** and there is **no unit-id string literal in game logic**. Watch."

**Action:** run live in the terminal:
```bash
# Linux / macOS / Git-Bash:
grep -rn '"witch-king"\|"gandalf"\|"sauron"\|"frodo"' option-b/internal/game option-b/internal/engine
```
```powershell
# Windows PowerShell (no grep) — easiest:
.\demo.ps1 check
# …or the raw equivalent:
Select-String -Path .\option-b\internal\game\*.go,.\option-b\internal\engine\*.go -Pattern '"witch-king"','"gandalf"','"sauron"','"frodo"'
```
> "Nothing in the logic. Behaviour comes from **config flags**. Here's the
> detection code…"

**Action:** open `internal/game/detection.go`, highlight:
```go
if m.Hops(n.Region, rbTrueRegion) <= EffectiveRange(n.Config, sauronActiveInMordor)
```
> "A Nazgul's range is `n.Config.DetectionRange` — config, not identity. The
> Witch-King is special only because its config says `detectionRange: 2` and
> `indestructible: true`. Adding a new unit is a single line in `units.json` — zero
> code change." *(Show `config/units.json` briefly.)*

> "One honesty note we put in the README: the spec text says 14 units but the config
> block defines 13. We load **exactly** what the config holds — which is the whole
> point of config-driven design — so the 14th unit would be a one-line edit."

### [2:30–3:15] Kafka topics — **K1**

**Action:** run (needs the stack up — `.\demo.ps1 up` first):
```bash
docker compose exec kafka kafka-topics --bootstrap-server kafka:29092 --describe | head -40
```
```powershell
# Windows PowerShell:
.\demo.ps1 describe
# …or raw (head -40 -> Select-Object -First 60):
docker compose exec kafka kafka-topics --bootstrap-server kafka:29092 --describe | Select-Object -First 60
```
> "All **ten** topics from the spec, with the right partitions and cleanup policy:
> `orders.raw` keyed by playerId with 3 partitions; the `events.*` topics keyed by
> unit, region and path with 6 partitions for per-entity ordering; `game.session`
> is **log-compacted** so a restarting instance reads the latest turn instantly;
> and the two secret channels — `ring.position` for Light only, `ring.detection`
> for Dark only — are single low-volume partitions."

### [3:15–3:55] Avro schemas — **K2**

**Action:** register them first (`.\demo.ps1 schemas`), then list:
```bash
curl -s http://localhost:8085/subjects | jq .
```
```powershell
# Windows PowerShell (no curl/jq needed):
.\demo.ps1 schemas      # registers all .avsc (run once)
.\demo.ps1 subjects     # lists them
# …or raw:
Invoke-RestMethod http://localhost:8085/subjects | ConvertTo-Json
```
> "Every event has an Avro schema registered in the Schema Registry under the
> `{topic}-value` naming strategy — `WorldStateSnapshot`, `RingBearerMoved`,
> `GameOver`, `DLQEntry`, and the rest. Reads and writes are schema-checked."

### [3:55–4:30] Schema evolution — **K3**

> "The spec requires a **backward-compatible V2**: add a nullable `routeRiskScore`
> to `OrderValidated`, and deploy it while V1 consumers keep running."

**Action:**
```bash
make schema-v2
curl -s http://localhost:8085/subjects/game.orders.validated-value/versions | jq .
```
```powershell
# Windows PowerShell (no make/jq needed):
.\demo.ps1 evolve
```
> "Two versions now coexist. Because the new field has `default: null`, a V1
> consumer reading a V2 record just ignores it — no errors. That's the live
> evolution check."

### [4:30–5:00] Validation rules — **K4 / K5** and hand-off

**Action:** open `internal/game/validation.go`, scroll the 8-rule switch.
> "Order validation — Topology 1 — is these **eight rules**, each returning the exact
> error code: `WRONG_TURN`, `NOT_YOUR_UNIT`, `PATH_BLOCKED`, and so on, with invalid
> orders going to the DLQ. And Topology 2 enriches routes with the `routeRiskScore`
> formula, which we reuse for the Light-Side analysis panel. Presenter B will now
> start a live game and show the most important property of the whole system —
> information hiding. Over to you."

**[Hand off on camera.]**
