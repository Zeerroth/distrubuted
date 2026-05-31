package game

import "rotr/internal/config"

// ApplyDamage is the config-driven damage/lifecycle transition (spec §18).
//
//	indestructible -> strength floors at 1, stays ACTIVE (never DESTROYED/RESPAWNING)
//	respawns       -> on fatal damage goes RESPAWNING (returns home after RespawnTurns)
//	otherwise      -> on fatal damage goes DESTROYED
//
// Nothing here reads the unit id; behaviour comes from cfg flags only.
func ApplyDamage(s UnitState, cfg config.UnitConfig, damage int) UnitState {
	raw := s.Strength - damage

	if cfg.Indestructible {
		if raw < 1 {
			raw = 1
		}
		s.Strength = raw
		s.Status = StatusActive
		return s
	}

	if raw <= 0 {
		s.Strength = 0
		if cfg.Respawns {
			s.Status = StatusRespawning
			s.RespawnTurns = cfg.RespawnTurns
			s.Region = ""
		} else {
			s.Status = StatusDestroyed
		}
		return s
	}

	s.Strength = raw
	return s
}

// AdvanceResult is the outcome of a single auto-advance step (spec §6 step 7).
type AdvanceResult string

const (
	AdvanceMoved        AdvanceResult = "MOVED"
	AdvanceBlocked      AdvanceResult = "BLOCKED"
	AdvanceRouteDone    AdvanceResult = "ROUTE_COMPLETE"
	AdvanceNoRoute      AdvanceResult = "NO_ROUTE"
	AdvanceNotActive    AdvanceResult = "NOT_ACTIVE"
)

// NextPathID returns the path the unit would traverse next, or "" if none.
func (s UnitState) NextPathID() (string, bool) {
	if s.RouteIdx < 0 || s.RouteIdx >= len(s.Route) {
		return "", false
	}
	return s.Route[s.RouteIdx], true
}

// AutoAdvance moves the unit one step along its route (spec §6 step 7).
// It consults the path's status to decide whether the move happens. The caller
// supplies the destination region for the next path (resolved via the map) and
// the current PathStatus. Returns the updated state and what occurred.
func AutoAdvance(s UnitState, nextPath PathState, destRegion string) (UnitState, AdvanceResult) {
	if s.Status != StatusActive {
		return s, AdvanceNotActive
	}
	pid, ok := s.NextPathID()
	if !ok {
		return s, AdvanceNoRoute
	}
	if pid != nextPath.ID {
		// caller passed the wrong path; treat as no route to be safe
		return s, AdvanceNoRoute
	}
	switch nextPath.Status {
	case PathBlocked:
		// unit stays; RouteBlocked emitted by caller
		return s, AdvanceBlocked
	case PathOpen, PathThreatened, PathTempOpen:
		s.Region = destRegion
		s.RouteIdx++
		if s.RouteIdx >= len(s.Route) {
			return s, AdvanceRouteDone
		}
		return s, AdvanceMoved
	default:
		return s, AdvanceBlocked
	}
}
