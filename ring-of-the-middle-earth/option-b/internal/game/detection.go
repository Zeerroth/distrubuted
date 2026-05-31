package game

import "rotr/internal/config"

// NazgulView is the minimal data the detection step needs about one Nazgul.
type NazgulView struct {
	ID     string
	Region string
	Config config.UnitConfig
}

// EffectiveRange applies the Eye of Sauron amplifier (spec §3.5 / §3.6): while
// Sauron is in Mordor and ACTIVE, EVERY Nazgul gains +1 detection range. This is
// applied by reading config flags / Sauron's position — Sauron is never sent an
// order (Q&A question 5).
func EffectiveRange(n config.UnitConfig, sauronActiveInMordor bool) int {
	r := n.DetectionRange
	if sauronActiveInMordor {
		r++
	}
	return r
}

// DetectionResult is the outcome of the per-turn detection check.
type DetectionResult struct {
	Detected       bool
	DetectedRegion string // ring bearer's true region, only meaningful if Detected
	ByNazgul       string // which Nazgul detected (first match), for logging
}

// RunDetection implements spec §3.6. Detection is SUPPRESSED for turns
// 1..hiddenUntilTurn (the Hidden Start). It also accounts for surveillance:
// crossing a path with surveillanceLevel>=1 exposes the bearer (handled at the
// movement step), so callers OR this result with that flag.
//
// `sauronActiveInMordor` must be computed by the caller from Sauron's live state
// (region == "mordor" && status == ACTIVE), again with no id literal in logic.
func RunDetection(turn, hiddenUntilTurn int, nazguls []NazgulView, rbTrueRegion string, sauronActiveInMordor bool, m *GameMap) DetectionResult {
	if turn <= hiddenUntilTurn {
		return DetectionResult{Detected: false}
	}
	for _, n := range nazguls {
		rng := EffectiveRange(n.Config, sauronActiveInMordor)
		if m.Hops(n.Region, rbTrueRegion) <= rng {
			return DetectionResult{Detected: true, DetectedRegion: rbTrueRegion, ByNazgul: n.ID}
		}
	}
	return DetectionResult{Detected: false}
}
