package tests

import (
	"testing"

	"rotr/internal/config"
	"rotr/internal/game"
)

// line map: a - b - c - d  (each hop = 1 edge). a..c is 2 hops.
func lineMap(t *testing.T) *game.GameMap {
	t.Helper()
	m, err := game.NewMap(
		[]game.Region{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "mordor"}},
		[]game.Path{
			{ID: "ab", From: "a", To: "b", Cost: 1},
			{ID: "bc", From: "b", To: "c", Cost: 1},
			{ID: "cd", From: "c", To: "d", Cost: 1},
		},
	)
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	return m
}

func nazgul(id, region string, rng int) game.NazgulView {
	return game.NazgulView{ID: id, Region: region, Config: config.UnitConfig{ID: id, DetectionRange: rng}}
}

// detection_test.go — mirrors RingBearerActorSpec (spec §24), graded under B4.

// 1. Nazgul (range=1) at 1 hop -> exposed=true.
func TestDetection_Range1At1Hop(t *testing.T) {
	m := lineMap(t)
	res := game.RunDetection(10, 3, []game.NazgulView{nazgul("n", "b", 1)}, "a", false, m)
	if !res.Detected {
		t.Fatal("expected detection at 1 hop with range 1")
	}
}

// 2. Nazgul (range=1) at 2 hops -> exposed=false.
func TestDetection_Range1At2Hops(t *testing.T) {
	m := lineMap(t)
	res := game.RunDetection(10, 3, []game.NazgulView{nazgul("n", "c", 1)}, "a", false, m)
	if res.Detected {
		t.Fatal("expected NO detection at 2 hops with range 1")
	}
}

// 3. Nazgul (range=2) at 2 hops -> exposed=true.
func TestDetection_Range2At2Hops(t *testing.T) {
	m := lineMap(t)
	res := game.RunDetection(10, 3, []game.NazgulView{nazgul("n", "c", 2)}, "a", false, m)
	if !res.Detected {
		t.Fatal("expected detection at 2 hops with range 2")
	}
}

// 4. Sauron active in Mordor + Nazgul (range=1) at 2 hops -> eff range=2 -> exposed.
func TestDetection_SauronAmplifier(t *testing.T) {
	m := lineMap(t)
	res := game.RunDetection(10, 3, []game.NazgulView{nazgul("n", "c", 1)}, "a", true /*sauronActiveInMordor*/, m)
	if !res.Detected {
		t.Fatal("expected detection: Sauron lifts range 1 -> 2 at 2 hops")
	}
}

// 5. Hidden start: detection suppressed on turns 1..hiddenUntilTurn.
func TestDetection_HiddenStartSuppresses(t *testing.T) {
	m := lineMap(t)
	res := game.RunDetection(2, 3, []game.NazgulView{nazgul("n", "a", 2)}, "a", false, m)
	if res.Detected {
		t.Fatal("detection must be suppressed during the hidden start (turn <= 3)")
	}
}
