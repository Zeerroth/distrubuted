package tests

import (
	"context"
	"testing"

	"rotr/internal/game"
	"rotr/internal/pipeline"
)

// pipeline2_test.go — 2 cases (spec §35).

// 1. Positive intercept window -> score > 0.
func TestPipeline2_PositiveWindow(t *testing.T) {
	// turnsToIntercept=1, rbTurnsToReach=3 -> window=+2 ; routeLength=5
	score := pipeline.InterceptScore(1, 3, 5)
	if score <= 0 {
		t.Fatalf("expected score > 0 for positive window, got %v", score)
	}
	if score != 1.0-1.0/5.0 {
		t.Fatalf("expected 0.8, got %v", score)
	}
}

// 2. Negative intercept window -> score = 0.0.
func TestPipeline2_NegativeWindow(t *testing.T) {
	// turnsToIntercept=5, rbTurnsToReach=2 -> window=-3
	score := pipeline.InterceptScore(5, 2, 5)
	if score != 0.0 {
		t.Fatalf("expected score 0.0 for negative window, got %v", score)
	}
}

// Extra: the concurrent runner produces one best entry per Nazgul.
func TestPipeline2_RunnerPicksBestPerNazgul(t *testing.T) {
	m := smallMap(t)
	cands := []pipeline.InterceptCandidate{
		{UnitID: "n1", NazgulRegion: "a", TargetRegion: "b", RBTurnsToReach: 5, RouteLength: 5},
		{UnitID: "n1", NazgulRegion: "a", TargetRegion: "c", RBTurnsToReach: 5, RouteLength: 5},
	}
	plan := pipeline.RunIntercept(context.Background(), cands, m)
	if len(plan.ByUnit) != 1 {
		t.Fatalf("expected 1 entry for n1, got %d", len(plan.ByUnit))
	}
	if plan.ByUnit[0].Score <= 0 {
		t.Fatalf("expected a positive best score, got %v", plan.ByUnit[0].Score)
	}
}

// keep the import used even if detection helpers move around
var _ = game.PathOpen
