package game

// Status is the unit lifecycle state (spec §18).
type Status string

const (
	StatusActive     Status = "ACTIVE"
	StatusDestroyed  Status = "DESTROYED"
	StatusRespawning Status = "RESPAWNING"
)

// UnitState is the mutable per-unit runtime state (spec §18 / §29 UnitSnapshot).
// Region is ALWAYS "" for the Ring Bearer in any shared/public state; its true
// region lives only in RingBearerState.
type UnitState struct {
	ID           string   `json:"id"`
	Region       string   `json:"region"`
	Strength     int      `json:"strength"`
	Status       Status   `json:"status"`
	RespawnTurns int      `json:"respawnTurns"`
	Route        []string `json:"route"`    // ordered path ids
	RouteIdx     int      `json:"routeIdx"` // index of NEXT path to traverse
	Cooldown     int      `json:"cooldown"`
}

// PathStatus is the path state-machine state (spec §19).
type PathStatus string

const (
	PathOpen        PathStatus = "OPEN"
	PathThreatened  PathStatus = "THREATENED"
	PathBlocked     PathStatus = "BLOCKED"
	PathTempOpen    PathStatus = "TEMPORARILY_OPEN"
)

// PathState is mutable per-path runtime state (spec §2.2 / §19).
type PathState struct {
	ID                string     `json:"id"`
	Status            PathStatus `json:"status"`
	SurveillanceLevel int        `json:"surveillanceLevel"` // 0..3
	TempOpenTurns     int        `json:"tempOpenTurns"`
	BlockedBy         string     `json:"blockedBy"` // unit id holding the block, or ""
	Corrupted         bool       `json:"corrupted"` // Saruman permanent corruption
}

// RegionState is mutable per-region runtime state (spec §20).
type RegionState struct {
	ID           string   `json:"id"`
	ControlledBy string   `json:"controlledBy"`
	ThreatLevel  int      `json:"threatLevel"`
	Fortified    bool     `json:"fortified"`
	FortifyTurns int      `json:"fortifyTurns"`
	UnitsPresent []string `json:"unitsPresent"` // unit ids physically present
}

// RingBearerState is owned by the single RingBearer authority (spec §21 / §29).
// TrueRegion is NEVER copied into any Dark-Side-visible structure.
type RingBearerState struct {
	TrueRegion         string   `json:"trueRegion"`
	Exposed            bool     `json:"exposed"`
	Route              []string `json:"route"`
	RouteIdx           int      `json:"routeIdx"`
	LastDetectedTurn   *int     `json:"lastDetectedTurn,omitempty"`
	LastDetectedRegion *string  `json:"lastDetectedRegion,omitempty"`
}

// NewUnitState builds the initial runtime state for a unit at its start region.
func NewUnitState(id, startRegion string, strength int) UnitState {
	return UnitState{
		ID:       id,
		Region:   startRegion,
		Strength: strength,
		Status:   StatusActive,
		Route:    nil,
		RouteIdx: 0,
	}
}
