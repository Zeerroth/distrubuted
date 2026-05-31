# Presenter C — Maia Dispatch · Path Mechanics · Combat · Fault Tolerance · Exactly-Once

**Target length: 5:00.** Covers rubric **B5** (Maia single-type dispatch — Demo
Scenario 2), **B6** (path blocking reverts when the blocker leaves), **B3**
(combat formula), **B2** (consumer-group rebalance — Demo Scenario 3), **K6**
(GameOver exactly once). Ends the video.

On-screen plan: the Command Map UI (View = Both) + a 2–3 pane terminal.

> **⚠ Windows / PowerShell.** This part needs the **full cluster** for Scenario 3:
> `.\demo.ps1 up-all` (builds the 3 Go nodes), then `.\demo.ps1 topics` and
> `.\demo.ps1 schemas`. The Maia/path/combat beats only need the local engine
> (`.\demo.ps1 run`). All bash commands below have a PowerShell form beside them.
> The fastest way to show the Maia/path beats is the **Scenarios tab** buttons —
> `Scenario 2a`, `2b`, `2c`, and `Win Drive` — which play the exact verified
> sequences automatically.

---

### [0:00–0:55] Maia dispatch — **Demo Scenario 2 / B5** (the other anti-generation check)

> "Thanks. The trickiest design point: Gandalf and Saruman both send the **same**
> order type — `MAIA_ABILITY` — but it does completely different things. A generated
> solution invents two order types; the correct one dispatches on **config**. Let me
> show the code first."

**Action:** open `internal/engine/engine.go`, scroll to Step 6 (the `MaiaAbility`
loop), highlight:
```go
case len(c.MaiaAbilityPaths) == 0 && ... :  // Gandalf -> OpenPath
case contains(c.MaiaAbilityPaths, ...) && !SarumanDisabled && ... :  // Saruman -> CorruptPath
```
> "Same order type. The branch is chosen by whether the unit's config has a
> `maiaAbilityPaths` allow-list — Saruman does, Gandalf doesn't. No names."

**Action:** click **Scenarios → Scenario 2a · Saruman Corrupts a Path** (or send
`MAIA_ABILITY` for `saruman` on `fords-of-isen-to-edoras` manually). It walks
Saruman to the Fords and corrupts the path.
> "Saruman corrupts `fords-of-isen-to-edoras`. The path's surveillance jumps to 3,
> **permanently** — a `PathCorrupted` event fires, purple on the map. From now on
> the Ring Bearer crossing it is exposed regardless of Nazgul positions, which is
> why Route 4's risk score went up in Presenter B's panel."

**Action:** click **Scenarios → Scenario 2b · Gandalf Opens a Blocked Path** (it
blocks `moria-to-lothlorien` with a Nazgûl, then opens it with Gandalf — same order
type).
> "Same order type for Gandalf turns a **blocked** path **temporarily open** for two
> turns — blue on the map — then it reverts. Different effect, one order type,
> chosen purely by config."

### [0:55–1:50] Path blocking reverts — **B6**

> "Path blocking requires **presence** — and a guard denies it outright."

**Action:** click **Scenarios → Scenario 2c · Guard Denies a Nazgûl Block** — Legolas
takes the Lothlórien endpoint, a Nazgûl deploys opposite and **tries** to block
`lothlorien-to-emyn-muil`; the block **fails** while the guard is present.
> "The FellowshipGuard at the endpoint denies the block — the path stays **OPEN**;
> the engine even emits a `BlockFailed` event. That's spec §2.4."

> "And the related rule (B6): if a Nazgûl *does* block a path with no guard, the
> block **reverts to OPEN** the moment that Nazgûl leaves the endpoint — the
> `ReconcileBlock` step." *(Show it in `internal/game/path.go`; you can demo it
> manually: BLOCK_PATH a clear path, then REDIRECT the blocker away and End Turn.)*

### [1:50–2:40] Combat — **B3**

> "Combat is a pure formula with terrain, fortification, leadership, and the
> ignore-fortress flag. The canonical example: an Uruk-hai **alone** can't take a
> fortified Minas Tirith…"

**Action (terminal):**
```bash
cd option-b && go test ./tests -run Combat -v
```
```powershell
# Windows PowerShell:
$env:Path += ";C:\Program Files\Go\bin"; cd option-b; go test ./tests -run Combat -v
```
> "All six combat cases pass: 5-versus-5 on plains is a repel; on a fortress the
> defender wins 5 to 7; the Uruk-hai ignores the **terrain** bonus but **not** the
> fortification; leadership gives Gimli +1 next to Aragorn so 5-plus-4 beats 5; and
> the indestructible Witch-King floors at strength 1 instead of dying. Every branch
> reads config flags — no unit ids."

### [2:40–4:05] Fault tolerance — **Demo Scenario 3 / B2**

> "Now the distributed payoff. We have three Go instances — go-1, go-2, go-3 — in one
> Kafka consumer group. I'll kill one **mid-game**."

**Setup (PowerShell):** `.\demo.ps1 up-all` (builds + starts go-1/2/3), then
`.\demo.ps1 topics` and `.\demo.ps1 schemas`. Light = http://localhost:8081/,
Dark = http://localhost:8082/.

**Action (terminal pane a — logs):**
```bash
docker compose logs -f go-1 go-2 go-3        # bash AND PowerShell (docker is the same)
```
**Pane c — kill a node:**
```bash
docker stop go-2                              # bash AND PowerShell
```
> "Watch the logs: Kafka detects go-2 left and triggers a **consumer-group
> rebalance**. go-2's partitions are reassigned to go-1 and go-3 within seconds —
> there's the rebalance line — and the game keeps running. I'll end a turn from the
> Light browser to prove it; it still works, served now by the surviving instances."

**Action:** end a turn in a browser; show it succeeds.
```bash
docker start go-2
```
> "Bring go-2 back. It **rejoins** the group and **rebuilds its KTable view** by
> replaying its assigned partitions from Kafka — no state was lost, because the
> state was never in go-2 to begin with. It lives in the broker. That's the Option B
> philosophy: stateless processes, stateful broker."

### [4:05–4:45] Exactly-once GameOver — **K6**

> "Last guarantee: the `GameOver` event must appear **exactly once**, even if the
> engine crashes at the worst moment."

**Action (terminal pane b):**
```bash
# bash:
docker compose exec kafka kafka-console-consumer --bootstrap-server kafka:29092 \
  --topic game.broadcast --from-beginning | grep --line-buffered GameOver
```
```powershell
# Windows PowerShell (grep -> Select-String):
docker compose exec kafka kafka-console-consumer --bootstrap-server kafka:29092 `
  --topic game.broadcast --from-beginning | Select-String GameOver
```
**Action:** drive the win with the UI **Scenarios → Win Drive** button (Ring Bearer
to Mount Doom + `DESTROY_RING`), then immediately `docker restart go-1` (the
producing instance).
> "I trigger the win and kill the engine the instant it produces. After restart, the
> consumer on `game.broadcast` shows **GameOver exactly once** — not zero, not two —
> because we produce it with `enable.idempotence=true`. There it is, a single
> record."

### [4:45–5:00] Wrap-up

> "To recap: a config-driven engine with no hardcoded unit ids; ten Kafka topics
> with schema evolution; provable information hiding pinned by the router tests
> (race-clean on CI); the same Maia order type dispatching two different effects by config;
> path blocking that respects presence; and a three-instance cluster that survives a
> node failure with exactly-once `GameOver`. Thanks for watching — we're happy to
> take questions."

**[End of video.]**

---

## Q&A quick-reference (spec §41) — be ready to point at these live
1. **Detection range, no `"witch-king"`:** `internal/game/detection.go` →
   `EffectiveRange(n.Config, …)`.
2. **Maia dispatch field:** `internal/engine/engine.go` Step 6 → `c.MaiaAbilityPaths`.
3. **Guard blocks a Nazgul block:** validation + `ReconcileBlock` in
   `internal/game/path.go`.
4. **RB position stripped:** `internal/router/router.go` `stripRingBearer` +
   `internal/api/hub.go` `stripBroadcast`.
5. **Sauron's passive applied where:** detection step, `sauronActiveInMordor()` in
   `engine.go` (identified by config flags, not id).
6/7. **Crash recovery:** rebuild from compacted `game.session` + assigned
   `game.events.*` partitions after rebalance.
8. **session compaction vs broadcast retention:** recover turn/world from the
   compacted `game.session`, not from `game.broadcast`.
