package tests

import (
	"testing"

	"rotr/internal/config"
	"rotr/internal/game"
)

// statemachine_test.go — PathActor + UnitActor transitions (spec §18/§19),
// graded under B5/B6 and the lifecycle parts of B3.

func TestPath_ThreatThenBlockThenRevert(t *testing.T) {
	p := game.PathState{ID: "p", Status: game.PathOpen}
	p = game.ThreatPath(p)
	if p.Status != game.PathThreatened {
		t.Fatalf("OPEN+Threat -> THREATENED, got %s", p.Status)
	}
	p = game.BlockPath(p, "nazgul-2")
	if p.Status != game.PathBlocked || p.BlockedBy != "nazgul-2" {
		t.Fatalf("THREATENED+Block -> BLOCKED by nazgul-2, got %s/%s", p.Status, p.BlockedBy)
	}
	// blocker leaves endpoint -> reverts to OPEN (spec §2.4)
	p = game.ReconcileBlock(p, false)
	if p.Status != game.PathOpen || p.BlockedBy != "" {
		t.Fatalf("BLOCKED + blocker absent -> OPEN, got %s/%s", p.Status, p.BlockedBy)
	}
}

func TestPath_GandalfTempOpenRevertsToBlocked(t *testing.T) {
	p := game.PathState{ID: "p", Status: game.PathBlocked, BlockedBy: "nazgul-3"}
	p = game.OpenPathGandalf(p)
	if p.Status != game.PathTempOpen || p.TempOpenTurns != 2 {
		t.Fatalf("BLOCKED+Gandalf -> TEMP_OPEN timer 2, got %s/%d", p.Status, p.TempOpenTurns)
	}
	p = game.TickTempOpen(p, true) // timer 2 -> 1
	p = game.TickTempOpen(p, true) // timer 1 -> 0, blocker present -> BLOCKED
	if p.Status != game.PathBlocked {
		t.Fatalf("TEMP_OPEN timer=0 with blocker -> BLOCKED, got %s", p.Status)
	}
}

func TestPath_SarumanCorruptionPermanent(t *testing.T) {
	p := game.PathState{ID: "fords-of-isen-to-edoras", Status: game.PathOpen}
	p = game.CorruptPathSaruman(p)
	if p.SurveillanceLevel != 3 || !p.Corrupted {
		t.Fatalf("Saruman corrupt -> surveillance 3 + corrupted, got %d/%v", p.SurveillanceLevel, p.Corrupted)
	}
}

func TestPath_SearchPathCapsAt3(t *testing.T) {
	p := game.PathState{ID: "p", Status: game.PathOpen}
	for i := 0; i < 5; i++ {
		p = game.SearchPath(p)
	}
	if p.SurveillanceLevel != 3 {
		t.Fatalf("SearchPath caps at 3, got %d", p.SurveillanceLevel)
	}
}

func TestUnit_RespawnAndDestroyAndIndestructible(t *testing.T) {
	// respawns=true -> RESPAWNING on fatal damage
	respawnCfg := config.UnitConfig{ID: "nazgul-2", Respawns: true, RespawnTurns: 3}
	s := game.UnitState{ID: "nazgul-2", Strength: 3, Status: game.StatusActive}
	out := game.ApplyDamage(s, respawnCfg, 5)
	if out.Status != game.StatusRespawning || out.RespawnTurns != 3 {
		t.Fatalf("respawns -> RESPAWNING(3), got %s/%d", out.Status, out.RespawnTurns)
	}

	// non-respawning -> DESTROYED
	plainCfg := config.UnitConfig{ID: "legolas"}
	out = game.ApplyDamage(game.UnitState{ID: "legolas", Strength: 3, Status: game.StatusActive}, plainCfg, 5)
	if out.Status != game.StatusDestroyed {
		t.Fatalf("non-respawning fatal -> DESTROYED, got %s", out.Status)
	}

	// indestructible -> floors at 1, ACTIVE
	indCfg := config.UnitConfig{ID: "sauron", Indestructible: true}
	out = game.ApplyDamage(game.UnitState{ID: "sauron", Strength: 5, Status: game.StatusActive}, indCfg, 99)
	if out.Strength != 1 || out.Status != game.StatusActive {
		t.Fatalf("indestructible -> 1/ACTIVE, got %d/%s", out.Strength, out.Status)
	}
}

func TestUnit_AutoAdvanceBlockedStays(t *testing.T) {
	s := game.UnitState{ID: "ring-bearer", Region: "a", Status: game.StatusActive, Route: []string{"ab"}, RouteIdx: 0}
	blocked := game.PathState{ID: "ab", Status: game.PathBlocked}
	out, r := game.AutoAdvance(s, blocked, "b")
	if r != game.AdvanceBlocked || out.Region != "a" {
		t.Fatalf("blocked path -> unit stays, got result=%s region=%s", r, out.Region)
	}
	open := game.PathState{ID: "ab", Status: game.PathOpen}
	out, r = game.AutoAdvance(s, open, "b")
	if r != game.AdvanceRouteDone || out.Region != "b" {
		t.Fatalf("open last path -> move + ROUTE_COMPLETE, got result=%s region=%s", r, out.Region)
	}
}
