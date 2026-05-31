package tests

import (
	"testing"

	"rotr/internal/game"
	"rotr/internal/pipeline"
)

// small synthetic map: a -- b -- c  (costs 1)
func smallMap(t *testing.T) *game.GameMap {
	t.Helper()
	m, err := game.NewMap(
		[]game.Region{
			{ID: "a", Terrain: game.TerrainPlains, StartThreat: 0},
			{ID: "b", Terrain: game.TerrainPlains, StartThreat: 2},
			{ID: "c", Terrain: game.TerrainPlains, StartThreat: 3},
		},
		[]game.Path{
			{ID: "p1", From: "a", To: "b", Cost: 1},
			{ID: "p2", From: "b", To: "c", Cost: 1},
		},
	)
	if err != nil {
		t.Fatalf("build map: %v", err)
	}
	return m
}

// pipeline1_test.go — 2 cases (spec §35).

// 1. Route with known threat and surveillance values -> correct riskScore.
func TestPipeline1_KnownRiskScore(t *testing.T) {
	m := smallMap(t)
	route, ok := pipeline.BuildRoute("r", m, "a", []string{"p1", "p2"})
	if !ok {
		t.Fatal("route did not build")
	}
	regions := map[string]game.RegionState{
		"a": {ID: "a", ThreatLevel: 0},
		"b": {ID: "b", ThreatLevel: 2},
		"c": {ID: "c", ThreatLevel: 3},
	}
	paths := map[string]game.PathState{
		"p1": {ID: "p1", Status: game.PathOpen, SurveillanceLevel: 1},
		"p2": {ID: "p2", Status: game.PathThreatened, SurveillanceLevel: 0},
	}
	// threat(b)+threat(c)=5 ; surveillance 1*3=3 ; threatened p2 -> 1*2=2 ; no nazgul
	// expected = 5 + 3 + 2 = 10
	got := pipeline.RouteRiskScore(route, m, paths, regions, nil)
	if got != 10 {
		t.Fatalf("expected riskScore 10, got %d", got)
	}
}

// 2. Nazgul within 2 hops -> proximity count adds correctly to the score.
func TestPipeline1_NazgulProximity(t *testing.T) {
	m := smallMap(t)
	route, _ := pipeline.BuildRoute("r", m, "a", []string{"p1", "p2"})
	regions := map[string]game.RegionState{"a": {}, "b": {}, "c": {}}
	paths := map[string]game.PathState{
		"p1": {ID: "p1", Status: game.PathOpen},
		"p2": {ID: "p2", Status: game.PathOpen},
	}
	base := pipeline.RouteRiskScore(route, m, paths, regions, nil)
	if base != 0 {
		t.Fatalf("expected baseline 0 with no threat/surveillance/nazgul, got %d", base)
	}
	// nazgul at "a" is on the route (0 hops, <= 2) -> proximity 1 -> +2
	withNazgul := pipeline.RouteRiskScore(route, m, paths, regions, []string{"a"})
	if withNazgul != base+2 {
		t.Fatalf("expected proximity to add 2 (got %d, base %d)", withNazgul, base)
	}
}
