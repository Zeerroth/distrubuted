package tests

import (
	"testing"

	"rotr/internal/config"
	"rotr/internal/game"
)

// helper: a combatant with a given strength and config flags.
func combatant(id string, strength int, leader bool, leaderBonus int, ignoresFortress, indestructible bool) game.Combatant {
	return game.Combatant{
		State: game.UnitState{ID: id, Strength: strength, Status: game.StatusActive},
		Config: config.UnitConfig{
			ID: id, Strength: strength,
			Leadership: leader, LeadershipBonus: leaderBonus,
			IgnoresFortress: ignoresFortress, Indestructible: indestructible,
		},
	}
}

// combat_test.go — 6 cases (spec §35).

// 1. Attacker(5) vs Defender(5, PLAINS) -> tie, attacker repelled.
func TestCombat_TieOnPlains(t *testing.T) {
	res := game.ResolveCombat(game.CombatInput{
		Attackers: []game.Combatant{combatant("a", 5, false, 0, false, false)},
		Defenders: []game.Combatant{combatant("d", 5, false, 0, false, false)},
		Terrain:   game.TerrainPlains,
	})
	if res.AttackerWon {
		t.Fatalf("expected attacker repelled, got win (%d vs %d)", res.AttackerPower, res.DefenderPower)
	}
	if res.AttackerPower != 5 || res.DefenderPower != 5 {
		t.Fatalf("expected 5 vs 5, got %d vs %d", res.AttackerPower, res.DefenderPower)
	}
}

// 2. Attacker(5) vs Defender(5, FORTRESS) -> defender wins (5 vs 7).
func TestCombat_FortressDefenderWins(t *testing.T) {
	res := game.ResolveCombat(game.CombatInput{
		Attackers: []game.Combatant{combatant("a", 5, false, 0, false, false)},
		Defenders: []game.Combatant{combatant("d", 5, false, 0, false, false)},
		Terrain:   game.TerrainFortress,
	})
	if res.AttackerWon || res.DefenderPower != 7 {
		t.Fatalf("expected defender win 5 vs 7, got %d vs %d won=%v", res.AttackerPower, res.DefenderPower, res.AttackerWon)
	}
}

// 3. UrukHai(5, ignoresFortress) vs Defender(5, FORTRESS) -> tie (5 vs 5).
func TestCombat_IgnoresFortressTerrain(t *testing.T) {
	res := game.ResolveCombat(game.CombatInput{
		Attackers: []game.Combatant{combatant("uruk", 5, false, 0, true, false)},
		Defenders: []game.Combatant{combatant("d", 5, false, 0, false, false)},
		Terrain:   game.TerrainFortress,
	})
	if res.AttackerWon || res.DefenderPower != 5 {
		t.Fatalf("expected tie 5 vs 5 (terrain skipped), got %d vs %d", res.AttackerPower, res.DefenderPower)
	}
}

// 4. UrukHai(5, ignoresFortress) vs Defender(5, FORTRESS, fortified) -> defender wins (5 vs 7).
func TestCombat_FortificationStillAppliesUnderIgnoresFortress(t *testing.T) {
	res := game.ResolveCombat(game.CombatInput{
		Attackers: []game.Combatant{combatant("uruk", 5, false, 0, true, false)},
		Defenders: []game.Combatant{combatant("d", 5, false, 0, false, false)},
		Terrain:   game.TerrainFortress,
		Fortified: true,
	})
	if res.AttackerWon || res.DefenderPower != 7 {
		t.Fatalf("expected defender win 5 vs 7 (fort applies), got %d vs %d", res.AttackerPower, res.DefenderPower)
	}
}

// 5. Leadership bonus applied correctly to co-located allies.
func TestCombat_LeadershipBonus(t *testing.T) {
	aragorn := combatant("aragorn", 5, true, 1, false, false)
	gimli := combatant("gimli", 3, false, 0, false, false)
	res := game.ResolveCombat(game.CombatInput{
		Attackers: []game.Combatant{aragorn, gimli},
		Defenders: []game.Combatant{combatant("d", 5, false, 0, false, false)},
		Terrain:   game.TerrainPlains,
	})
	// gimli effective = 3+1 = 4; aragorn = 5; total 9 vs 5
	if res.AttackerPower != 9 {
		t.Fatalf("expected attacker power 9 (5 + (3+1)), got %d", res.AttackerPower)
	}
	if !res.AttackerWon || res.Damage != 4 {
		t.Fatalf("expected attacker win damage 4, got won=%v dmg=%d", res.AttackerWon, res.Damage)
	}
}

// 6. Indestructible unit: strength floors at 1, stays ACTIVE.
func TestCombat_IndestructibleFloorsAtOne(t *testing.T) {
	cfg := config.UnitConfig{ID: "witch-king", Indestructible: true}
	s := game.UnitState{ID: "witch-king", Strength: 5, Status: game.StatusActive}
	out := game.ApplyDamage(s, cfg, 10) // fatal damage
	if out.Strength != 1 || out.Status != game.StatusActive {
		t.Fatalf("expected strength=1 ACTIVE, got strength=%d status=%s", out.Strength, out.Status)
	}
}
