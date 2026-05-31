# Presenter B — Live Gameplay · Information Hiding · Detection · Analysis

**Target length: 5:00.** Covers rubric **B10** (full HVH game), **B7/A9**
(information hiding — Demo Scenario 1), **B4** (detection + Sauron amplifier +
hidden start), **B8** (analysis pipelines).

On-screen plan: the Command Map UI in **View = Both** (Light board left, Dark board
right in one window) + a terminal. Use **End Turn ▶** to advance turns.

> **⚠ Windows / PowerShell.** Run the UI locally: `.\demo.ps1 run` → open
> **http://localhost:8080/** → View = Both → Connect. Terminal commands below are
> bash; their **PowerShell** form is shown beside each. The easiest way to drive
> this whole part is the **Scenarios tab → “Scenario 1 · Information Hiding”**
> button, which plays the exact sequence automatically while you narrate.

---

### [0:00–0:30] Setup recap

> "Thanks. This single window shows the **Light Side** on the left and the **Dark
> Side** on the right. Both are plain HTML with Server-Sent-Events — no React, no
> Vue, as the spec requires. They see one shared game because all state lives in
> the engine (and in the cluster, in Kafka). Let's play."

**Tip:** you can click **Scenarios → Scenario 1 · Information Hiding** to auto-play
the next two sections, or drive it manually as written below.

### [0:30–1:30] Start a game and move the Ring Bearer — **B10**

**Action (Light browser):** submit an `ASSIGN_ROUTE` for `ring-bearer`:
`shire-to-bree, bree-to-weathertop`. Then click **End Turn ▶** a couple of times.

> "I assign the Ring Bearer the Fellowship route and end the turn. On the **Light**
> browser you can see the Ring Bearer's **true position** update — Shire, then Bree,
> then Weathertop — because Light is privileged. Now look at the **Dark** browser
> at the same moment…"

**Action:** point at the Dark browser's Ring Bearer line.
> "…it says **'??? hidden from the Shadow.'** Dark has no idea where the Ring Bearer
> is. That asymmetry is the heart of the game."

### [1:30–2:45] Information hiding — **Demo Scenario 1 / B7 / A9**

> "Let's prove the hiding is real, not just a UI trick. First, the state endpoint."

**Action (terminal):**
```bash
# bash:
curl -s "http://localhost:8080/game/state?side=dark"  | jq '.units["ring-bearer"].region, .ringBearerRegion'
curl -s "http://localhost:8080/game/state?side=light" | jq '.units["ring-bearer"].region, .ringBearerRegion'
```
```powershell
# Windows PowerShell (no curl/jq):
(Invoke-RestMethod "http://localhost:8080/game/state?side=dark").ringBearerRegion   # -> "" (gizli)
(Invoke-RestMethod "http://localhost:8080/game/state?side=light").ringBearerRegion  # -> gerçek bölge
```
> "Dark gets an **empty string** for the Ring Bearer's region; Light gets the real
> region. Same snapshot, stripped at the delivery edge."

**Action:** open `internal/router/router.go`, highlight the `Route` switch and
`stripRingBearer`.
> "This is the **single enforcement point**, exactly as the spec asks. `game.ring.position`
> — the RingBearerMoved event — is sent to the Light channel and **never** the Dark
> one. The broadcast snapshot goes to both, but the Dark copy runs through
> `stripRingBearer`, which blanks the region. And the cache's `DarkView.RingBearerRegion`
> is hard-wired to empty — we pin that with a `go test -race` test. Let me run it."

**Action (terminal):**
```bash
cd option-b && go test -race ./tests -run Router
```
```powershell
# Windows PowerShell:
$env:Path += ";C:\Program Files\Go\bin"; cd option-b
go test -race ./tests -run Router
```
> "Green, with the race detector on. The Dark Side **cannot** learn the position
> through any channel."

> **✅ `-race` works now.** A portable **mingw-w64 gcc** is installed at
> `%USERPROFILE%\mingw-dist\mingw64\bin` and added to PATH, and Go auto-enables CGO
> when gcc is present — so `-race` runs (verified: `ok rotr/tests`). Easiest:
> ```powershell
> .\demo.ps1 testrace          # full suite under the race detector (B7)
> ```
> Note: `-run Router` matches only the two `TestRouter_*` tests; the **concurrent**
> race test is `TestCache_DarkViewRingBearerAlwaysEmpty`. To run all three
> information-hiding tests under `-race`, use `-run 'Router|Cache'` or just the full
> suite. (If you move to a machine without gcc, `.\demo.ps1 test` still passes the
> logic without `-race`.)

### [2:45–3:50] Detection — **B4** (the one legitimate reveal)

> "There's exactly one way Dark *can* learn it: **detection**. Watch."

**Action (Light):** move the Ring Bearer to `weathertop` (already there or end a
turn). **Action:** move the **Witch-King** toward Bree (detection range 2), end the
turn past turn 3.

> "I bring the Witch-King within its detection range — range **2**, from config —
> and end the turn. Now the **Dark** browser lights up with **RING_BEARER_DETECTED
> at weathertop**, and only now does Dark see a position. The **Light** browser
> never receives that detection event."

> "Two subtleties the rubric checks. First, the **hidden start**: for the first
> three turns detection is fully suppressed — no matter how close a Nazgul is. We
> just passed turn 3, so it's active. Second, the **Eye of Sauron**: while Sauron
> sits in Mordor, every Nazgul gets **+1** range — the Witch-King's 2 becomes 3 —
> and Sauron is never sent an order. It's a passive read in the detection step."

**Action:** open `internal/game/detection.go`, point at `EffectiveRange` and the
`turn <= hiddenUntilTurn` guard.
> "All config-driven: `EffectiveRange(cfg, sauronActiveInMordor)`. And here's the
> detection test suite — range-1-at-1-hop true, range-1-at-2-hops false,
> range-2-at-2-hops true, and the Sauron-amplifier case."

```powershell
# Windows PowerShell:
$env:Path += ";C:\Program Files\Go\bin"; cd option-b; go test ./tests -run Detection -v
```

### [3:50–4:40] Analysis pipelines — **B8**

> "Each side gets a concurrent analysis pipeline. Light asks 'which route is
> safest?'"

**Action (Light board):** click the **Analysis** button (top-right of the board).
> "Pipeline 1 fans the four canonical routes across four goroutine workers and
> scores each with the spec's risk formula — region threat, surveillance, blocked
> and threatened paths, and Nazgul proximity — then ranks them. Route 4, the
> southern corridor, scores higher risk once Saruman corrupts a path, which
> Presenter C will trigger."

**Action (Dark board):** click the **Analysis** button.
> "Dark runs Pipeline 2 — interception. For each Nazgul and each route region it
> computes an intercept window and a score. Both pipelines use a context with a
> two-second timeout and return partial results rather than blocking the turn."

### [4:40–5:00] Hand-off

> "So: a full game runs end to end, the Ring Bearer's position is provably hidden
> from the Shadow, detection is the only reveal, and both sides get live concurrent
> analysis. Presenter C will now show the Maia abilities, path blocking, and what
> happens when we kill a node mid-game. Over to you."

**[Hand off on camera.]**
