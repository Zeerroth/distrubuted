# Demo Video — Filming Plan (3 presenters)

A single ~15-minute video covering the project, split into three ~5-minute parts —
one per team member — matching the assessment demo (spec §40) and the rubric. Each
presenter has their own script file with **word-for-word narration**, **on-screen
actions**, **timestamps**, and the **rubric criteria** each beat proves.

| Part | Presenter | Theme | Rubric covered |
|---|---|---|---|
| 1 | **Presenter A** | Intro · Architecture · Config-driven design · Kafka topics, schemas & schema evolution | A2/B1, K1, K2, K3, K4, K5 |
| 2 | **Presenter B** | End-to-end gameplay · **Information hiding** (Scenario 1) · Detection + Sauron + hidden start · Analysis pipelines | B10, B7/A9, B4, B8 |
| 3 | **Presenter C** | **Maia dispatch + path mechanics** (Scenario 2) · Combat · **Fault tolerance + exactly-once** (Scenario 3) · Wrap-up | B5, B6, B3, B2, K6 |

## The Command Map UI (use this to drive every scenario on camera)

The engine serves a polished SVG command map. The easiest, most reliable way to
film is the **Both** view + the **one-click Scenario buttons**, which play the
exact engine-verified order sequences automatically with live logging.

- **Local (simplest, recommended for the video):** one shell →
  `cd option-b && go run ./cmd/server -config ../config -ui ../ui`, then open
  **http://localhost:8080/**. Set **View = Both**, click **Connect**.
- **Docker (for Scenario 3 only):** `make up`, open **http://localhost:8081/**
  (View = Both still works — each instance has full state via Kafka).
- The **Scenarios tab** has: `Information Hiding`, `Saruman Corrupts a Path`,
  `Gandalf Opens a Blocked Path`, `Guard Denies a Nazgûl Block`, `Win Drive`.
  Click one and narrate while it animates. **Reset ↺** between takes.
- The **Order tab** lets you drive anything manually (build routes by clicking
  regions). **End Turn ▶** advances a turn.

## Shared setup (do this BEFORE recording — keep it off-camera or in a 20s cold open)

1. Start the engine + UI as above (local `go run`, or `make up` for the cluster).
2. Open **http://localhost:8080/** (or `:8081`), **View = Both**, **Connect**.
   For the side-by-side info-hiding shot this single window shows Light (Ring
   Bearer visible) and Dark (Ring Bearer hidden) at once.
3. For Scenario 3, also open a **terminal** with three panes: (a) `make logs`,
   (b) `kafka-console-consumer` on `game.broadcast`, (c) a shell for `docker stop/start`.
4. Have an editor open to the files you'll point at (no scrolling hunts on camera):
   `internal/game/detection.go`, `internal/engine/engine.go`,
   `internal/router/router.go`, `internal/game/validation.go`.

## Recording tips
- Screen-record at 1080p; face-cam optional (picture-in-picture bottom-right).
- One presenter drives the screen while speaking; hand off explicitly on camera
  ("…and now Presenter B will take us through a live game").
- The instructor controls inputs in the real demo; in the *video* you drive them,
  but narrate as if explaining, not reciting.
- Keep each part within time; total ≤ 15 min. Pre-recorded output is for the video
  submission — the live graded demo is separate and not pre-recorded (spec §40).

## The three demo scenarios (reference)
1. **Information hiding** — Dark never sees the Ring Bearer's region; only a
   detection event reveals it (Presenter B).
2. **Maia dispatch + path mechanics** — same `MAIA_ABILITY` type → Gandalf opens
   vs Saruman corrupts; guard blocks a Nazgul (Presenter C).
3. **Fault tolerance + exactly-once** — kill `go-2`, observe rebalance; `GameOver`
   exactly once on `game.broadcast` (Presenter C).
