# Presenter B — Live Gameplay · Information Hiding · Detection · Analysis

**Target length: 5:00.** Covers rubric **B10** (full HVH game), **B7/A9**
(information hiding — Demo Scenario 1), **B4** (detection + Sauron amplifier +
hidden start), **B8** (analysis pipelines).

On-screen plan: two browsers side by side (Light left @ :8081, Dark right @ :8082)
+ a terminal. Use the **End Turn ▶** button to advance turns on demand.

---

### [0:00–0:30] Setup recap

> "Thanks. On the left is the **Light Side** browser, on the right the **Dark Side**.
> Both are plain HTML with Server-Sent-Events — no React, no Vue, as the spec
> requires. They're connected to two different Go instances, but they see one shared
> game because all state lives in Kafka. Let's play."

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
curl -s "http://localhost:8082/game/state?side=dark" | jq '.units["ring-bearer"].region, .ringBearerRegion'
curl -s "http://localhost:8081/game/state?side=light" | jq '.units["ring-bearer"].region, .ringBearerRegion'
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
> "Green, with the race detector on. The Dark Side **cannot** learn the position
> through any channel."

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
> range-2-at-2-hops true, and the Sauron-amplifier case." *(Show `go test ./tests -run Detection`.)*

### [3:50–4:40] Analysis pipelines — **B8**

> "Each side gets a concurrent analysis pipeline. Light asks 'which route is
> safest?'"

**Action (Light):** click **Refresh analysis**.
> "Pipeline 1 fans the four canonical routes across four goroutine workers and
> scores each with the spec's risk formula — region threat, surveillance, blocked
> and threatened paths, and Nazgul proximity — then ranks them. Route 4, the
> southern corridor, scores higher risk once Saruman corrupts a path, which
> Presenter C will trigger."

**Action (Dark):** click **Refresh analysis**.
> "Dark runs Pipeline 2 — interception. For each Nazgul and each route region it
> computes an intercept window and a score. Both pipelines use a context with a
> two-second timeout and return partial results rather than blocking the turn."

### [4:40–5:00] Hand-off

> "So: a full game runs end to end, the Ring Bearer's position is provably hidden
> from the Shadow, detection is the only reveal, and both sides get live concurrent
> analysis. Presenter C will now show the Maia abilities, path blocking, and what
> happens when we kill a node mid-game. Over to you."

**[Hand off on camera.]**
