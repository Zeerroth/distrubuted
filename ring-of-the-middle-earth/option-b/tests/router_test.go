package tests

import (
	"sync"
	"testing"

	"rotr/internal/config"
	"rotr/internal/game"
	"rotr/internal/router"
)

// router_test.go — 3 cases, all intended for `go test -race` (spec §35 / B7).

// 1. WorldStateSnapshot with ring-bearer region set -> Dark Side receives
//    currentRegion="", Light Side receives the real value.
func TestRouter_BroadcastStripsRingBearerForDark(t *testing.T) {
	r := router.NewEventRouter(8)
	snap := &router.WorldStateSnapshot{
		Turn: 5,
		Units: map[string]game.UnitState{
			router.RingBearerUnitID: {ID: router.RingBearerUnitID, Region: "weathertop", Strength: 1, Status: game.StatusActive},
		},
		RingBearerRegion: "weathertop",
	}
	r.Route(router.Event{Topic: router.TopicBroadcast, Type: "WorldStateSnapshot", Turn: 5, Snapshot: snap})

	light := <-r.LightCh
	dark := <-r.DarkCh

	if got := light.Snapshot.Units[router.RingBearerUnitID].Region; got != "weathertop" {
		t.Fatalf("light side should see real region, got %q", got)
	}
	if got := dark.Snapshot.Units[router.RingBearerUnitID].Region; got != "" {
		t.Fatalf("dark side must see empty region, got %q", got)
	}
	if dark.Snapshot.RingBearerRegion != "" {
		t.Fatalf("dark side top-level RingBearerRegion must be empty, got %q", dark.Snapshot.RingBearerRegion)
	}
	// the original (light) snapshot must NOT have been mutated by stripping
	if snap.Units[router.RingBearerUnitID].Region != "weathertop" {
		t.Fatalf("stripping mutated the original snapshot")
	}
}

// 2. RingBearerMoved (game.ring.position) -> never reaches the Dark Side channel.
func TestRouter_RingPositionNeverReachesDark(t *testing.T) {
	r := router.NewEventRouter(8)
	r.Route(router.Event{Topic: router.TopicRingPosition, Type: "RingBearerMoved", Turn: 3, Region: "bree"})

	if len(r.LightCh) != 1 {
		t.Fatalf("light side should have received RingBearerMoved, len=%d", len(r.LightCh))
	}
	if len(r.DarkCh) != 0 {
		t.Fatalf("dark side must NOT receive RingBearerMoved, len=%d", len(r.DarkCh))
	}
}

// 3. cache.DarkView.RingBearerRegion is always "" after any cache update, even
//    under concurrent writers/readers (race detector).
func TestCache_DarkViewRingBearerAlwaysEmpty(t *testing.T) {
	cfgs := map[string]config.UnitConfig{router.RingBearerUnitID: {ID: router.RingBearerUnitID}}
	c := router.NewWorldStateCache(cfgs)

	var wg sync.WaitGroup
	// concurrent writers applying snapshots with a REAL ring-bearer region
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(turn int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				c.ApplySnapshot(router.WorldStateSnapshot{
					Turn:             turn,
					RingBearerRegion: "mount-doom", // real value on the light snapshot
				})
			}
		}(w)
	}
	// concurrent readers asserting the invariant
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if got := c.DarkView().RingBearerRegion; got != "" {
					t.Errorf("DarkView.RingBearerRegion must always be empty, got %q", got)
					return
				}
			}
		}()
	}
	wg.Wait()

	// and the Light view DOES hold the real value
	if got := c.LightView().RingBearerRegion; got != "mount-doom" {
		t.Fatalf("light view should hold real region, got %q", got)
	}
}
