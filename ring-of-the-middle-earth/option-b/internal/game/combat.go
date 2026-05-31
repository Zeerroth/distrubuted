package game

import "rotr/internal/config"

// Combatant pairs a unit's mutable state with its immutable config. Combat reads
// ONLY config booleans (Leadership, IgnoresFortress, Indestructible) — never the
// unit id. This is what spec A2/B1 + Q&A question 1 verify.
type Combatant struct {
	State  UnitState
	Config config.UnitConfig
}

// CombatInput describes one AttackRegion resolution (spec §4.1).
type CombatInput struct {
	Attackers []Combatant
	Defenders []Combatant
	Terrain   string // defender region's terrain
	Fortified bool   // defender region's fortified flag
}

// CombatResult is the outcome of resolving one attack.
type CombatResult struct {
	AttackerPower int
	DefenderPower int
	AttackerWon   bool
	Damage        int // damage dealt to defenders when AttackerWon; else 0
}

// terrainBonus returns the defender's terrain bonus (spec §4.1).
func terrainBonus(terrain string) int {
	switch terrain {
	case TerrainFortress:
		return 2
	case TerrainMountains:
		return 1
	default:
		return 0
	}
}

// effectiveStrength applies leadership: every co-located ally (same group) gains
// +leadershipBonus from each OTHER unit in the group whose config has
// Leadership=true. Driven purely by config flags.
func effectiveStrength(u Combatant, group []Combatant) int {
	eff := u.State.Strength
	for _, other := range group {
		if other.State.ID == u.State.ID {
			continue
		}
		if other.Config.Leadership {
			eff += other.Config.LeadershipBonus
		}
	}
	return eff
}

func groupPower(group []Combatant) int {
	total := 0
	for _, u := range group {
		total += effectiveStrength(u, group)
	}
	return total
}

// attackersIgnoreFortress reports whether the terrain bonus must be skipped:
// true when ANY attacker has IgnoresFortress (spec §4.1 — applies only when
// attacking; a defending Uruk-hai still benefits from its fortress).
func attackersIgnoreFortress(attackers []Combatant) bool {
	for _, a := range attackers {
		if a.Config.IgnoresFortress {
			return true
		}
	}
	return false
}

// ResolveCombat implements spec §4.1 exactly.
func ResolveCombat(in CombatInput) CombatResult {
	attackerPower := groupPower(in.Attackers)

	defenderPower := groupPower(in.Defenders)
	if !attackersIgnoreFortress(in.Attackers) {
		defenderPower += terrainBonus(in.Terrain) // skipped if ignoresFortress
	}
	if in.Fortified {
		defenderPower += 2 // fortification ALWAYS applies, even with ignoresFortress
	}

	res := CombatResult{AttackerPower: attackerPower, DefenderPower: defenderPower}
	if attackerPower > defenderPower {
		res.AttackerWon = true
		res.Damage = attackerPower - defenderPower
	}
	return res
}
