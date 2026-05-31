package game

// Path state machine (spec §19). All transitions are pure functions on PathState
// so they are trivially unit-testable and free of side effects.

// BlockPath: OPEN/THREATENED -> BLOCKED, held by unit `by`.
// A BLOCKED path reverts (see ReconcileBlock) once `by` leaves the endpoint.
func BlockPath(p PathState, by string) PathState {
	if p.Status == PathOpen || p.Status == PathThreatened {
		p.Status = PathBlocked
		p.BlockedBy = by
	}
	return p
}

// ThreatPath: OPEN -> THREATENED.
func ThreatPath(p PathState) PathState {
	if p.Status == PathOpen {
		p.Status = PathThreatened
	}
	return p
}

// ClearPath: THREATENED/BLOCKED -> OPEN.
func ClearPath(p PathState) PathState {
	if p.Status == PathThreatened || p.Status == PathBlocked {
		p.Status = PathOpen
		p.BlockedBy = ""
	}
	return p
}

// OpenPathGandalf: BLOCKED -> TEMPORARILY_OPEN with timer=2 (Gandalf MaiaAbility).
// Caller is responsible for verifying Gandalf adjacency + cooldown beforehand.
func OpenPathGandalf(p PathState) PathState {
	if p.Status == PathBlocked {
		p.Status = PathTempOpen
		p.TempOpenTurns = 2
	}
	return p
}

// CorruptPathSaruman: any status -> surveillanceLevel=3, permanent (Saruman
// MaiaAbility). The path status itself is unchanged; only surveillance + the
// Corrupted flag are set. Crossing it later forces exposed=true (spec §3.5).
func CorruptPathSaruman(p PathState) PathState {
	p.SurveillanceLevel = 3
	p.Corrupted = true
	return p
}

// SearchPath: any status -> surveillanceLevel += 1 (max 3). Dark Side SearchPath.
func SearchPath(p PathState) PathState {
	if p.SurveillanceLevel < 3 {
		p.SurveillanceLevel++
	}
	return p
}

// ReconcileBlock reverts a BLOCKED path when its blocking unit is no longer at an
// endpoint (spec §2.4 / §6 step 3): "A path remains BLOCKED only while the
// blocking unit stays at one of its endpoint regions." `blockerPresent` is true
// when BlockedBy is still at an endpoint.
func ReconcileBlock(p PathState, blockerPresent bool) PathState {
	if p.Status == PathBlocked && !blockerPresent {
		p.Status = PathOpen
		p.BlockedBy = ""
	}
	return p
}

// TickTempOpen decrements a TEMPORARILY_OPEN timer (spec §6 step 9).
// On timer=0: -> BLOCKED if a blocker is still present, else -> OPEN.
func TickTempOpen(p PathState, blockerPresent bool) PathState {
	if p.Status != PathTempOpen {
		return p
	}
	if p.TempOpenTurns > 0 {
		p.TempOpenTurns--
	}
	if p.TempOpenTurns == 0 {
		if blockerPresent {
			p.Status = PathBlocked
		} else {
			p.Status = PathOpen
			p.BlockedBy = ""
		}
	}
	return p
}
