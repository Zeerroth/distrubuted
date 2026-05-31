package game

import "rotr/internal/config"

// ValidationContext is the read-only state the validator consults. It mirrors the
// KTables of Kafka Streams Topology 1 (spec §11): TurnKTable, UnitKTable,
// PathKTable, RegionKTable.
type ValidationContext struct {
	CurrentTurn int
	Configs     map[string]config.UnitConfig
	Units       map[string]UnitState
	Paths       map[string]PathState
	Regions     map[string]RegionState
	Map         *GameMap
	// PlayerSide maps a playerId to its side (FREE_PEOPLES / SHADOW). The Light
	// player owns FREE_PEOPLES units, the Dark player owns SHADOW units.
	PlayerSide map[string]string
	// SeenUnits holds unit ids that already have a validated order THIS turn,
	// for the duplicate-order rule. The caller adds to it as orders pass.
	SeenUnits map[string]bool
}

// ValidationResult is the outcome. Valid==false carries one ErrorCode (spec §5.4).
type ValidationResult struct {
	Valid     bool
	ErrorCode string
}

func ok() ValidationResult      { return ValidationResult{Valid: true} }
func fail(c string) ValidationResult { return ValidationResult{Valid: false, ErrorCode: c} }

// Validate runs Topology 1's 8 rules in order (spec §11). The first failing rule
// wins. NOTE: every branch reads config flags / state, never a unit-id literal.
func Validate(o Order, ctx ValidationContext) ValidationResult {
	cfg, known := ctx.Configs[o.UnitID]

	// Rule 8 (checked first so a duplicate is caught regardless of other issues):
	// same unitId appears more than once this turn.
	if ctx.SeenUnits[o.UnitID] {
		return fail(ErrDuplicateUnitOrder)
	}

	// Rule 1: order.turn must match current turn.
	if o.Turn != ctx.CurrentTurn {
		return fail(ErrWrongTurn)
	}

	// Rule 2: unit must belong to the submitting player's side.
	if !known || ctx.PlayerSide[o.PlayerID] != cfg.Side {
		return fail(ErrNotYourUnit)
	}

	switch o.OrderType {
	case OrderAssignRoute, OrderRedirectUnit:
		paths := o.PathIDs
		if o.OrderType == OrderRedirectUnit {
			paths = o.NewPathIDs
		}
		// Rule 4: every path must exist in the graph (route must be well-formed).
		for _, pid := range paths {
			if _, exists := ctx.Map.Paths[pid]; !exists {
				return fail(ErrInvalidPath)
			}
		}
		// Rule 3: the NEXT path the unit would traverse must not be BLOCKED.
		if len(paths) > 0 {
			if p, exists := ctx.Paths[paths[0]]; exists && p.Status == PathBlocked {
				return fail(ErrPathBlocked)
			}
		}

	case OrderBlockPath, OrderSearchPath:
		// Rule 5: the unit must be in one of the path's endpoint regions.
		from, to, exists := ctx.Map.Endpoints(o.PathID)
		if !exists {
			return fail(ErrInvalidPath)
		}
		reg := ctx.Units[o.UnitID].Region
		if reg != from && reg != to {
			return fail(ErrUnitNotAdjacent)
		}

	case OrderAttackRegion:
		// Rule 6: target must be adjacent to the unit AND enemy-controlled.
		reg := ctx.Units[o.UnitID].Region
		if _, adj := ctx.Map.PathBetween(reg, o.TargetRegion); !adj {
			return fail(ErrInvalidTarget)
		}
		target, exists := ctx.Regions[o.TargetRegion]
		if !exists || target.ControlledBy == cfg.Side {
			return fail(ErrInvalidTarget)
		}

	case OrderMaiaAbility:
		// Rule 7: ability must not be on cooldown.
		if ctx.Units[o.UnitID].Cooldown > 0 {
			return fail(ErrAbilityOnCooldown)
		}
	}

	return ok()
}
