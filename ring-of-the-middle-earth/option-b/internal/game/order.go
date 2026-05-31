package game

// Order is the union of all order types (spec §5.3). Only the fields relevant to
// OrderType are populated.
type Order struct {
	OrderType    string   `json:"orderType"`
	PlayerID     string   `json:"playerId"`
	UnitID       string   `json:"unitId"`
	Turn         int      `json:"turn"`
	PathIDs      []string `json:"pathIds,omitempty"`      // ASSIGN_ROUTE
	NewPathIDs   []string `json:"newPathIds,omitempty"`   // REDIRECT_UNIT
	TargetPathID string   `json:"targetPathId,omitempty"` // MAIA_ABILITY
	PathID       string   `json:"pathId,omitempty"`       // BLOCK_PATH / SEARCH_PATH
	TargetRegion string   `json:"targetRegion,omitempty"` // ATTACK/REINFORCE/DEPLOY
}

// Order type string constants (spec §5.3).
const (
	OrderAssignRoute     = "ASSIGN_ROUTE"
	OrderRedirectUnit    = "REDIRECT_UNIT"
	OrderDestroyRing     = "DESTROY_RING"
	OrderMaiaAbility     = "MAIA_ABILITY"
	OrderBlockPath       = "BLOCK_PATH"
	OrderSearchPath      = "SEARCH_PATH"
	OrderAttackRegion    = "ATTACK_REGION"
	OrderReinforceRegion = "REINFORCE_REGION"
	OrderFortifyRegion   = "FORTIFY_REGION"
	OrderDeployNazgul    = "DEPLOY_NAZGUL"
)

// Error codes (spec §5.4).
const (
	ErrWrongTurn            = "WRONG_TURN"
	ErrNotYourUnit          = "NOT_YOUR_UNIT"
	ErrInvalidPath          = "INVALID_PATH"
	ErrPathBlocked          = "PATH_BLOCKED"
	ErrUnitNotAdjacent      = "UNIT_NOT_ADJACENT"
	ErrInvalidTarget        = "INVALID_TARGET"
	ErrDuplicateUnitOrder   = "DUPLICATE_UNIT_ORDER"
	ErrAbilityOnCooldown    = "ABILITY_ON_COOLDOWN"
	ErrMaiaDisabled         = "MAIA_DISABLED"
	ErrDestroyConditionUnmet = "DESTROY_CONDITION_NOT_MET"
)
