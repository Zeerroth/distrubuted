package router

import (
	"sync"

	"rotr/internal/config"
	"rotr/internal/game"
)

// LightSideView is the Light Side's privileged view (spec §29).
type LightSideView struct {
	RingBearerRegion string   // the TRUE region — Light Side only
	AssignedRoute    []string
	RouteIdx         int
}

// DarkSideView is the Dark Side's view (spec §29). RingBearerRegion is ALWAYS ""
// — NO code path ever sets it to a real value. Enforced in ApplySnapshot and
// verified by router_test.go with -race.
type DarkSideView struct {
	RingBearerRegion   string // ALWAYS ""
	LastDetectedRegion string
	LastDetectedTurn   int
}

// WorldStateCache is the in-process projection of world state (spec §29). It is
// concurrency-safe; the CacheManager goroutine owns writes, readers take RLock.
type WorldStateCache struct {
	mu          sync.RWMutex
	Turn        int
	Units       map[string]game.UnitState
	Regions     map[string]game.RegionState
	Paths       map[string]game.PathState
	UnitConfigs map[string]config.UnitConfig // read-only after startup
	lightView   LightSideView
	darkView    DarkSideView
}

// NewWorldStateCache builds an empty cache seeded with the read-only configs.
func NewWorldStateCache(cfgs map[string]config.UnitConfig) *WorldStateCache {
	return &WorldStateCache{
		Units:       map[string]game.UnitState{},
		Regions:     map[string]game.RegionState{},
		Paths:       map[string]game.PathState{},
		UnitConfigs: cfgs,
	}
}

// ApplySnapshot ingests a Light-Side broadcast (the unstripped one) and updates
// both views. The Dark view's RingBearerRegion is hard-wired to "" here — this
// is the invariant the race test pins down.
func (c *WorldStateCache) ApplySnapshot(snap WorldStateSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Turn = snap.Turn
	c.Units = snap.Units
	c.Regions = snap.Regions
	c.Paths = snap.Paths

	c.lightView.RingBearerRegion = snap.RingBearerRegion // real value, Light only
	c.darkView.RingBearerRegion = ""                     // INVARIANT: never real
}

// SetDetected records a detection for the Dark Side view (region IS revealed once
// detection fires — that is the only legitimate channel).
func (c *WorldStateCache) SetDetected(region string, turn int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.darkView.LastDetectedRegion = region
	c.darkView.LastDetectedTurn = turn
}

// LightView returns a copy of the Light Side view.
func (c *WorldStateCache) LightView() LightSideView {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lightView
}

// DarkView returns a copy of the Dark Side view. RingBearerRegion is always "".
func (c *WorldStateCache) DarkView() DarkSideView {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.darkView
}
