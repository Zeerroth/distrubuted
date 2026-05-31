// Package config loads the SHARED, authoritative game configuration.
//
// Design principle (spec §3.1 / §27): every unit shares ONE implementation.
// Behaviour is entirely driven by these config fields — there is NO unit-id
// string literal anywhere in game logic. Adding a unit is a config edit, not a
// code change. Q&A question 1 verifies this live.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// UnitConfig mirrors spec §27 exactly. Game logic branches on these BOOLEAN and
// NUMERIC fields (Indestructible, Respawns, DetectionRange, Maia, ...), never on
// ID or Name.
type UnitConfig struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Class            string   `json:"class"`
	Side             string   `json:"side"`
	StartRegion      string   `json:"start"`
	Strength         int      `json:"strength"`
	Leadership       bool     `json:"leadership"`
	LeadershipBonus  int      `json:"leadershipBonus"`
	Indestructible   bool     `json:"indestructible"`
	DetectionRange   int      `json:"detectionRange"`
	Respawns         bool     `json:"respawns"`
	RespawnTurns     int      `json:"respawnTurns"`
	Maia             bool     `json:"maia"`
	MaiaAbilityPaths []string `json:"maiaAbilityPaths"`
	IgnoresFortress  bool     `json:"ignoresFortress"`
	CanFortify       bool     `json:"canFortify"`
	Cooldown         int      `json:"cooldown"`
}

// Side constants — string values, not hardcoded unit identities.
const (
	SideFreePeoples = "FREE_PEOPLES"
	SideShadow      = "SHADOW"
)

// GameConfig is the loaded, validated configuration.
type GameConfig struct {
	Units               map[string]UnitConfig // id -> config (read-only after startup)
	UnitOrder           []string              // stable load order for deterministic iteration
	HiddenUntilTurn     int                   // detection suppressed for turns 1..HiddenUntilTurn
	MaxTurns            int
	TurnDurationSeconds int
}

type rawUnitsFile struct {
	HiddenUntilTurn     int          `json:"hiddenUntilTurn"`
	MaxTurns            int          `json:"maxTurns"`
	TurnDurationSeconds int          `json:"turnDurationSeconds"`
	Units               []UnitConfig `json:"units"`
}

// Load reads the units config file (config/units.json) and indexes it by id.
func Load(unitsPath string) (*GameConfig, error) {
	b, err := os.ReadFile(unitsPath)
	if err != nil {
		return nil, fmt.Errorf("read units config %q: %w", unitsPath, err)
	}
	var raw rawUnitsFile
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse units config: %w", err)
	}
	if len(raw.Units) == 0 {
		return nil, fmt.Errorf("units config %q contains no units", unitsPath)
	}

	cfg := &GameConfig{
		Units:               make(map[string]UnitConfig, len(raw.Units)),
		UnitOrder:           make([]string, 0, len(raw.Units)),
		HiddenUntilTurn:     raw.HiddenUntilTurn,
		MaxTurns:            raw.MaxTurns,
		TurnDurationSeconds: raw.TurnDurationSeconds,
	}
	for _, u := range raw.Units {
		if _, dup := cfg.Units[u.ID]; dup {
			return nil, fmt.Errorf("duplicate unit id %q in config", u.ID)
		}
		cfg.Units[u.ID] = u
		cfg.UnitOrder = append(cfg.UnitOrder, u.ID)
	}
	return cfg, nil
}

// UnitsBySide returns the config ids belonging to a side, in load order.
func (g *GameConfig) UnitsBySide(side string) []string {
	out := make([]string, 0)
	for _, id := range g.UnitOrder {
		if g.Units[id].Side == side {
			out = append(out, id)
		}
	}
	return out
}
