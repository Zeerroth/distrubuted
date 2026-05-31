// Package engine runs the authoritative 13-step turn processing (spec §6). It is
// the Go equivalent of Option A's WorldStateActor + GameSessionActor. State lives
// here in-process; in the 3-instance deployment this state is rebuilt from Kafka
// KTables (spec §26). All branching is config-driven — no unit-id literals.
package engine

import (
	"encoding/json"
	"sync"

	"rotr/internal/bus"
	"rotr/internal/config"
	"rotr/internal/game"
	"rotr/internal/router"
)

// GameState is the full authoritative state.
type GameState struct {
	Turn            int
	Units           map[string]game.UnitState
	Paths           map[string]game.PathState
	Regions         map[string]game.RegionState
	RB              game.RingBearerState
	Over            bool
	Winner          string
	Cause           string
	SarumanDisabled bool
}

// Engine owns the state and the per-turn order queue.
type Engine struct {
	mu         sync.Mutex
	cfg        *config.GameConfig
	gmap       *game.GameMap
	bus        bus.Bus
	state      GameState
	pending    []game.Order
	seen       map[string]bool
	playerSide map[string]string
	bearerID   string // resolved by class, not hardcoded
}

// NewEngine seeds initial state from config + map.
func NewEngine(cfg *config.GameConfig, gmap *game.GameMap, b bus.Bus) *Engine {
	e := &Engine{
		cfg:        cfg,
		gmap:       gmap,
		bus:        b,
		seen:       map[string]bool{},
		playerSide: map[string]string{"light": config.SideFreePeoples, "dark": config.SideShadow},
	}
	e.seed()
	return e
}

func (e *Engine) seed() {
	st := GameState{
		Turn:    1,
		Units:   map[string]game.UnitState{},
		Paths:   map[string]game.PathState{},
		Regions: map[string]game.RegionState{},
	}
	for id, r := range e.gmap.Regions {
		st.Regions[id] = game.RegionState{ID: id, ControlledBy: r.StartControl, ThreatLevel: r.StartThreat}
	}
	for id := range e.gmap.Paths {
		st.Paths[id] = game.PathState{ID: id, Status: game.PathOpen}
	}
	for _, id := range e.cfg.UnitOrder {
		c := e.cfg.Units[id]
		u := game.NewUnitState(id, c.StartRegion, c.Strength)
		if c.Class == "RingBearer" { // identify the bearer by CLASS, not id
			e.bearerID = id
			st.RB = game.RingBearerState{TrueRegion: c.StartRegion}
			u.Region = "" // public state always blank for the bearer
		} else {
			reg := st.Regions[c.StartRegion]
			reg.UnitsPresent = append(reg.UnitsPresent, id)
			st.Regions[c.StartRegion] = reg
		}
		st.Units[id] = u
	}
	e.state = st
}

// ---- order intake ----

// Submit validates an order against current state (Topology 1 rules) and queues
// it if valid. Invalid orders return the error code (the caller produces a DLQ
// entry). Valid orders are appended and the unit marked seen (duplicate rule).
func (e *Engine) Submit(o game.Order) game.ValidationResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	ctx := game.ValidationContext{
		CurrentTurn: e.state.Turn,
		Configs:     e.cfg.Units,
		Units:       e.state.Units,
		Paths:       e.state.Paths,
		Regions:     e.state.Regions,
		Map:         e.gmap,
		PlayerSide:  e.playerSide,
		SeenUnits:   e.seen,
	}
	res := game.Validate(o, ctx)
	if res.Valid {
		e.pending = append(e.pending, o)
		e.seen[o.UnitID] = true
	}
	return res
}

// ---- helpers (all config-driven) ----

func (e *Engine) isNazgul(id string) bool { return e.cfg.Units[id].Class == "Nazgul" }
func (e *Engine) isMaia(id string) bool   { return e.cfg.Units[id].Maia }

// isSauron identifies Sauron by CONFIG FLAGS only: the indestructible Shadow Maia.
// (A cleaner design would add an explicit `amplifiesDetection` flag; see ARCH doc.)
func (e *Engine) isSauron(id string) bool {
	c := e.cfg.Units[id]
	return c.Maia && c.Side == config.SideShadow && c.Indestructible
}

func (e *Engine) sauronActiveInMordor() bool {
	for id, u := range e.state.Units {
		if e.isSauron(id) && u.Status == game.StatusActive && u.Region == "mordor" {
			return true
		}
	}
	return false
}

func (e *Engine) nazgulViews() []game.NazgulView {
	var out []game.NazgulView
	for _, id := range e.cfg.UnitOrder {
		if e.isNazgul(id) {
			u := e.state.Units[id]
			if u.Status == game.StatusActive {
				out = append(out, game.NazgulView{ID: id, Region: u.Region, Config: e.cfg.Units[id]})
			}
		}
	}
	return out
}

// unitAtEndpoint reports whether unit `id` currently sits at an endpoint of path.
func (e *Engine) unitAtEndpoint(id, pathID string) bool {
	from, to, ok := e.gmap.Endpoints(pathID)
	if !ok {
		return false
	}
	r := e.state.Units[id].Region
	return r == from || r == to
}

// enemyGuardAtEndpoints reports whether an ACTIVE enemy combat unit (anything but
// the Ring Bearer) of the side OPPOSING `blockingSide` is present at either
// endpoint of the path — which denies a block (spec §2.4).
func (e *Engine) enemyGuardAtEndpoints(pathID, blockingSide string) bool {
	from, to, ok := e.gmap.Endpoints(pathID)
	if !ok {
		return false
	}
	for _, rid := range []string{from, to} {
		for _, uid := range e.state.Regions[rid].UnitsPresent {
			c := e.cfg.Units[uid]
			if c.Side != blockingSide && c.Class != "RingBearer" && e.state.Units[uid].Status == game.StatusActive {
				return true
			}
		}
	}
	return false
}

// Reset re-seeds the game to its initial state and clears the order queue. Used
// by POST /game/reset so a scenario can be re-run cleanly between video takes.
func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pending = nil
	e.seen = map[string]bool{}
	e.seed()
}

// ---- the 13-step turn (spec §6) ----

// ProcessTurn executes one full turn end and returns the snapshot that was
// broadcast. Steps map 1:1 to spec §6.
func (e *Engine) ProcessTurn() router.WorldStateSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state.Over {
		return e.snapshotLocked()
	}
	turn := e.state.Turn
	destroyRingSubmitted := false

	// Step 1: orders already collected in e.pending.
	// Step 2: AssignRoute / RedirectUnit.
	for _, o := range e.pending {
		switch o.OrderType {
		case game.OrderAssignRoute:
			u := e.state.Units[o.UnitID]
			u.Route = append([]string{}, o.PathIDs...)
			u.RouteIdx = 0
			e.state.Units[o.UnitID] = u
			if o.UnitID == e.bearerID {
				e.state.RB.Route = append([]string{}, o.PathIDs...)
				e.state.RB.RouteIdx = 0
			}
		case game.OrderRedirectUnit:
			u := e.state.Units[o.UnitID]
			u.Route = append([]string{}, o.NewPathIDs...)
			u.RouteIdx = 0
			e.state.Units[o.UnitID] = u
			if o.UnitID == e.bearerID {
				e.state.RB.Route = append([]string{}, o.NewPathIDs...)
				e.state.RB.RouteIdx = 0
			}
		case game.OrderDestroyRing:
			destroyRingSubmitted = true
		}
	}

	// Step 3: BlockPath / SearchPath.
	for _, o := range e.pending {
		switch o.OrderType {
		case game.OrderBlockPath:
			// A FellowshipGuard (any enemy combat unit) at an endpoint prevents the
			// block — the Nazgul must defeat it first (spec §2.4).
			if e.enemyGuardAtEndpoints(o.PathID, e.cfg.Units[o.UnitID].Side) {
				e.emit("game.events.path", o.PathID, map[string]any{
					"type": "BlockFailed", "pathId": o.PathID, "unitId": o.UnitID,
					"reason": "GUARD_PRESENT", "turn": turn})
				break
			}
			p := e.state.Paths[o.PathID]
			p = game.BlockPath(p, o.UnitID)
			e.state.Paths[o.PathID] = p
		case game.OrderSearchPath:
			p := e.state.Paths[o.PathID]
			p = game.SearchPath(p)
			e.state.Paths[o.PathID] = p
		}
	}
	// reconcile blocks: a BLOCKED path reverts if its blocker left its endpoint.
	for id, p := range e.state.Paths {
		if p.Status == game.PathBlocked {
			present := p.BlockedBy != "" && e.unitAtEndpoint(p.BlockedBy, id) && e.state.Units[p.BlockedBy].Status == game.StatusActive
			e.state.Paths[id] = game.ReconcileBlock(p, present)
		}
	}

	// Step 4/5: Reinforce / Deploy / Fortify.
	for _, o := range e.pending {
		if o.OrderType == game.OrderFortifyRegion && e.cfg.Units[o.UnitID].CanFortify {
			r := e.state.Regions[e.state.Units[o.UnitID].Region]
			r.Fortified = true
			r.FortifyTurns = 2
			e.state.Regions[r.ID] = r
		}
		if o.OrderType == game.OrderDeployNazgul && e.isNazgul(o.UnitID) {
			u := e.state.Units[o.UnitID]
			if u.Status == game.StatusActive {
				e.moveUnit(o.UnitID, o.TargetRegion)
			}
		}
	}

	// Step 6: MaiaAbility (Gandalf OpenPath vs Saruman CorruptPath) — SAME order
	// type, dispatched purely by which unit/config sent it.
	for _, o := range e.pending {
		if o.OrderType != game.OrderMaiaAbility || !e.isMaia(o.UnitID) {
			continue
		}
		if e.state.Units[o.UnitID].Cooldown > 0 {
			continue
		}
		c := e.cfg.Units[o.UnitID]
		p := e.state.Paths[o.TargetPathID]
		switch {
		// Gandalf-style: ability has NO fixed path list -> OpenPath a BLOCKED path.
		case len(c.MaiaAbilityPaths) == 0 && e.unitAtEndpoint(o.UnitID, o.TargetPathID):
			p = game.OpenPathGandalf(p)
			e.setCooldown(o.UnitID, c.Cooldown)
		// Saruman-style: ability has a path allow-list -> CorruptPath (permanent).
		case contains(c.MaiaAbilityPaths, o.TargetPathID) && !e.state.SarumanDisabled && e.unitAtEndpoint(o.UnitID, o.TargetPathID):
			p = game.CorruptPathSaruman(p)
			e.emit("game.events.path", o.TargetPathID, map[string]any{"type": "PathCorrupted", "pathId": o.TargetPathID, "turn": turn})
			e.setCooldown(o.UnitID, c.Cooldown)
		}
		e.state.Paths[o.TargetPathID] = p
	}

	// Step 7: auto-advance every unit with a route (Ring Bearer included).
	for _, id := range e.cfg.UnitOrder {
		e.advanceUnit(id, turn)
	}

	// Step 8: AttackRegion combat.
	e.resolveCombat(turn)

	// Step 9: decrement TEMPORARILY_OPEN timers.
	for id, p := range e.state.Paths {
		if p.Status == game.PathTempOpen {
			present := p.BlockedBy != "" && e.unitAtEndpoint(p.BlockedBy, id)
			e.state.Paths[id] = game.TickTempOpen(p, present)
		}
	}

	// Step 10: decrement fortification timers.
	for id, r := range e.state.Regions {
		if r.Fortified && r.FortifyTurns > 0 {
			r.FortifyTurns--
			if r.FortifyTurns == 0 {
				r.Fortified = false
			}
			e.state.Regions[id] = r
		}
	}

	// Step 11: decrement respawn + cooldown counters.
	for id, u := range e.state.Units {
		if u.Status == game.StatusRespawning {
			if u.RespawnTurns > 0 {
				u.RespawnTurns--
			}
			if u.RespawnTurns == 0 {
				c := e.cfg.Units[id]
				u.Status = game.StatusActive
				u.Strength = c.Strength
				u.Region = c.StartRegion // returns home at full strength
			}
		}
		if u.Cooldown > 0 {
			u.Cooldown--
		}
		e.state.Units[id] = u
	}

	// Step 12: detection (suppressed during the hidden start).
	det := game.RunDetection(turn, e.cfg.HiddenUntilTurn, e.nazgulViews(), e.state.RB.TrueRegion, e.sauronActiveInMordor(), e.gmap)
	if det.Detected {
		e.state.RB.Exposed = true
		e.state.RB.LastDetectedTurn = &turn
		reg := det.DetectedRegion
		e.state.RB.LastDetectedRegion = &reg
		// detection event -> Dark Side ONLY topic.
		e.emit("game.ring.detection", "dark", map[string]any{"type": "RingBearerDetected", "regionId": reg, "turn": turn})
	}

	// Step 13: evaluate win/draw, broadcast snapshot, reset exposed.
	e.evaluateWin(turn, destroyRingSubmitted)
	snap := e.snapshotLocked()
	e.emitSnapshot(snap)

	e.state.RB.Exposed = false
	e.pending = nil
	e.seen = map[string]bool{}
	if !e.state.Over {
		e.state.Turn++
	}
	return snap
}

// advanceUnit moves a unit one step along its route (spec §6 step 7), handling
// the Ring Bearer's hidden position and surveillance exposure.
func (e *Engine) advanceUnit(id string, turn int) {
	u := e.state.Units[id]
	isBearer := id == e.bearerID
	var route []string
	var idx int
	curRegion := u.Region
	if isBearer {
		route = e.state.RB.Route
		idx = e.state.RB.RouteIdx
		curRegion = e.state.RB.TrueRegion
	} else {
		route = u.Route
		idx = u.RouteIdx
	}
	if u.Status != game.StatusActive || idx < 0 || idx >= len(route) {
		return
	}
	pid := route[idx]
	p := e.state.Paths[pid]
	from, to, _ := e.gmap.Endpoints(pid)
	dest := to
	if curRegion == to {
		dest = from
	}
	switch p.Status {
	case game.PathBlocked:
		e.emit("game.events.unit", id, map[string]any{"type": "RouteBlocked", "unitId": id, "pathId": pid, "turn": turn})
		return
	case game.PathOpen, game.PathThreatened, game.PathTempOpen:
		if isBearer {
			e.state.RB.TrueRegion = dest
			e.state.RB.RouteIdx = idx + 1
			// RingBearerMoved -> Light Side ONLY topic.
			e.emit("game.ring.position", "light", map[string]any{"type": "RingBearerMoved", "trueRegion": dest, "turn": turn})
			// surveillance exposure (spec §6 step 7).
			if p.SurveillanceLevel >= 1 && turn > e.cfg.HiddenUntilTurn {
				e.state.RB.Exposed = true
				e.emit("game.ring.detection", "dark", map[string]any{"type": "RingBearerSpotted", "pathId": pid, "turn": turn})
			}
			// bearer's public unit entry stays blank
			u.RouteIdx = idx + 1
			e.state.Units[id] = u
		} else {
			e.moveUnit(id, dest)
			uu := e.state.Units[id]
			uu.RouteIdx = idx + 1
			e.state.Units[id] = uu
			e.emit("game.events.unit", id, map[string]any{"type": "UnitMoved", "unitId": id, "to": dest, "turn": turn})
		}
	}
}

// moveUnit relocates a non-bearer unit and maintains region membership.
func (e *Engine) moveUnit(id, dest string) {
	u := e.state.Units[id]
	if u.Region != "" {
		from := e.state.Regions[u.Region]
		from.UnitsPresent = remove(from.UnitsPresent, id)
		e.state.Regions[u.Region] = from
	}
	u.Region = dest
	e.state.Units[id] = u
	d := e.state.Regions[dest]
	d.UnitsPresent = appendUnique(d.UnitsPresent, id)
	e.state.Regions[dest] = d
}

// resolveCombat groups AttackRegion orders by target and resolves each (spec §8).
func (e *Engine) resolveCombat(turn int) {
	targets := map[string][]string{} // target region -> attacker unit ids
	for _, o := range e.pending {
		if o.OrderType == game.OrderAttackRegion {
			targets[o.TargetRegion] = append(targets[o.TargetRegion], o.UnitID)
		}
	}
	for target, attackerIDs := range targets {
		side := e.cfg.Units[attackerIDs[0]].Side
		var attackers, defenders []game.Combatant
		for _, id := range attackerIDs {
			u := e.state.Units[id]
			if u.Status == game.StatusActive {
				attackers = append(attackers, game.Combatant{State: u, Config: e.cfg.Units[id]})
			}
		}
		for _, id := range e.state.Regions[target].UnitsPresent {
			u := e.state.Units[id]
			if u.Status == game.StatusActive && e.cfg.Units[id].Side != side {
				defenders = append(defenders, game.Combatant{State: u, Config: e.cfg.Units[id]})
			}
		}
		reg := e.state.Regions[target]
		res := game.ResolveCombat(game.CombatInput{
			Attackers: attackers, Defenders: defenders,
			Terrain:   e.gmap.Regions[target].Terrain, Fortified: reg.Fortified,
		})
		e.emit("game.events.region", target, map[string]any{"type": "BattleResolved", "regionId": target, "attackerWon": res.AttackerWon, "turn": turn})
		if res.AttackerWon {
			e.applyCombatDamage(defenders, res.Damage)
			reg.ControlledBy = side
			e.state.Regions[target] = reg
			// Isengard falling to Free Peoples permanently disables Saruman (spec §6/§20).
			if target == "isengard" && side == config.SideFreePeoples {
				e.state.SarumanDisabled = true
			}
			// attackers occupy the region
			for _, id := range attackerIDs {
				if e.state.Units[id].Status == game.StatusActive {
					e.moveUnit(id, target)
				}
			}
		} else {
			for _, id := range attackerIDs {
				u := e.state.Units[id]
				if u.Status == game.StatusActive {
					e.state.Units[id] = game.ApplyDamage(u, e.cfg.Units[id], 1)
				}
			}
		}
	}
}

func (e *Engine) applyCombatDamage(defenders []game.Combatant, damage int) {
	for _, d := range defenders {
		if damage <= 0 {
			break
		}
		cur := e.state.Units[d.State.ID]
		before := cur.Strength
		e.state.Units[d.State.ID] = game.ApplyDamage(cur, d.Config, damage)
		damage -= before // spread remaining damage to the next defender
		// drop destroyed/respawning units out of region membership
		nu := e.state.Units[d.State.ID]
		if nu.Status != game.StatusActive && cur.Region != "" {
			r := e.state.Regions[cur.Region]
			r.UnitsPresent = remove(r.UnitsPresent, d.State.ID)
			e.state.Regions[cur.Region] = r
		}
	}
}

func (e *Engine) evaluateWin(turn int, destroyRingSubmitted bool) {
	// Light Side win.
	if e.state.RB.TrueRegion == "mount-doom" && destroyRingSubmitted {
		shadowPresent := false
		for _, id := range e.state.Regions["mount-doom"].UnitsPresent {
			if e.cfg.Units[id].Side == config.SideShadow && e.state.Units[id].Status == game.StatusActive {
				shadowPresent = true
			}
		}
		if !shadowPresent {
			e.gameOver(config.SideFreePeoples, "RING_DESTROYED", turn)
			return
		}
	}
	// Dark Side win: a Nazgul co-located with an EXPOSED Ring Bearer.
	if e.state.RB.Exposed {
		for _, n := range e.nazgulViews() {
			if n.Region == e.state.RB.TrueRegion {
				e.gameOver(config.SideShadow, "RING_BEARER_CAPTURED", turn)
				return
			}
		}
	}
	// Draw.
	if turn >= e.cfg.MaxTurns {
		e.gameOver("NONE", "DRAW", turn)
	}
}

func (e *Engine) gameOver(winner, cause string, turn int) {
	e.state.Over = true
	e.state.Winner = winner
	e.state.Cause = cause
	// GameOver is produced with idempotence in the Kafka bus (spec §13).
	e.emit("game.broadcast", "game-over", map[string]any{"type": "GameOver", "winner": winner, "cause": cause, "turn": turn})
}

func (e *Engine) setCooldown(id string, cd int) {
	u := e.state.Units[id]
	u.Cooldown = cd
	e.state.Units[id] = u
}

// ---- snapshot + emit ----

func (e *Engine) snapshotLocked() router.WorldStateSnapshot {
	units := make(map[string]game.UnitState, len(e.state.Units))
	for id, u := range e.state.Units {
		units[id] = u
	}
	regions := make(map[string]game.RegionState, len(e.state.Regions))
	for id, r := range e.state.Regions {
		regions[id] = r
	}
	paths := make(map[string]game.PathState, len(e.state.Paths))
	for id, p := range e.state.Paths {
		paths[id] = p
	}
	snap := router.WorldStateSnapshot{
		Turn:             e.state.Turn,
		Units:            units,
		Regions:          regions,
		Paths:            paths,
		RingBearerRegion: e.state.RB.TrueRegion, // Light-Side copy; stripped for Dark
		Over:             e.state.Over,
		Winner:           e.state.Winner,
		Cause:            e.state.Cause,
	}
	if e.state.RB.LastDetectedRegion != nil {
		snap.RingLastDetectedRegion = *e.state.RB.LastDetectedRegion
	}
	if e.state.RB.LastDetectedTurn != nil {
		snap.RingLastDetectedTurn = *e.state.RB.LastDetectedTurn
	}
	return snap
}

// Snapshot returns the current snapshot (thread-safe) for the HTTP /game/state.
func (e *Engine) Snapshot() router.WorldStateSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snapshotLocked()
}

func (e *Engine) emitSnapshot(snap router.WorldStateSnapshot) {
	b, _ := json.Marshal(map[string]any{"type": "WorldStateSnapshot", "snapshot": snap})
	_ = e.bus.Produce("game.broadcast", "world", b)
}

func (e *Engine) emit(topic, key string, payload map[string]any) {
	b, _ := json.Marshal(payload)
	_ = e.bus.Produce(topic, key, b)
}

// ---- small slice helpers ----

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
func remove(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}
func appendUnique(s []string, v string) []string {
	if contains(s, v) {
		return s
	}
	return append(s, v)
}
