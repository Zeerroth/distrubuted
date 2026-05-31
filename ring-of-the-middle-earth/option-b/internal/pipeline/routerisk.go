package pipeline

import (
	"context"
	"sort"
	"sync"
	"time"

	"rotr/internal/game"
)

// RouteRiskScore is the PURE risk function (spec §12 / §32). Identical to
// Topology 2's formula:
//
//	riskScore = sum(region.threatLevel for each destination region)
//	          + sum(path.surveillanceLevel for each path) * 3
//	          + count(BLOCKED paths)    * 5
//	          + count(THREATENED paths) * 2
//	          + nazgulProximityCount    * 2
//
// nazgulProximityCount = number of Nazgul within 2 graph hops of ANY region on
// the route.
func RouteRiskScore(r Route, m *game.GameMap, paths map[string]game.PathState, regions map[string]game.RegionState, nazgulRegions []string) int {
	score := 0
	for _, regionID := range r.DestRegks {
		score += regions[regionID].ThreatLevel
	}
	blocked, threatened := 0, 0
	for _, pid := range r.PathIDs {
		p := paths[pid]
		score += p.SurveillanceLevel * 3
		switch p.Status {
		case game.PathBlocked:
			blocked++
		case game.PathThreatened:
			threatened++
		}
	}
	score += blocked*5 + threatened*2

	prox := 0
	for _, nr := range nazgulRegions {
		for _, regionID := range r.Regions {
			if m.Hops(nr, regionID) <= 2 {
				prox++
				break
			}
		}
	}
	score += prox * 2
	return score
}

// RankedRoute is one scored entry in the Light-Side analysis result.
type RankedRoute struct {
	RouteID   string   `json:"routeId"`
	RiskScore int      `json:"riskScore"`
	Warnings  []string `json:"warnings"`
}

// RankedRouteList is the Pipeline 1 output (spec §32).
type RankedRouteList struct {
	Routes      []RankedRoute `json:"routes"`
	Recommended string        `json:"recommended"`
	Warnings    []string      `json:"warnings"`
	Partial     bool          `json:"partial"` // true if the 2s timeout truncated work
}

// RunRouteRisk is the concurrent runner (spec §32): a dispatcher feeds a buffered
// channel (cap 20), 4 workers score routes, an aggregator ranks them. Honors
// context cancellation (or-done) and a 2s timeout returning a partial result.
func RunRouteRisk(ctx context.Context, candidates []Route, m *game.GameMap, paths map[string]game.PathState, regions map[string]game.RegionState, nazgulRegions []string) RankedRouteList {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	const workers = 4
	in := make(chan Route, 20)
	out := make(chan RankedRoute)

	// dispatcher
	go func() {
		defer close(in)
		for _, c := range candidates {
			select {
			case in <- c:
			case <-ctx.Done():
				return
			}
		}
	}()

	// workers
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for r := range in {
				score := RouteRiskScore(r, m, paths, regions, nazgulRegions)
				rr := RankedRoute{RouteID: r.ID, RiskScore: score, Warnings: routeWarnings(r, paths)}
				select {
				case out <- rr:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() { wg.Wait(); close(out) }()

	// aggregator
	res := RankedRouteList{}
	collected := 0
	for {
		select {
		case rr, ok := <-out:
			if !ok {
				goto done
			}
			res.Routes = append(res.Routes, rr)
			collected++
		case <-ctx.Done():
			res.Partial = true
			goto done
		}
	}
done:
	if collected < len(candidates) && len(candidates) > 0 {
		res.Partial = true
	}
	sort.SliceStable(res.Routes, func(i, j int) bool {
		return res.Routes[i].RiskScore < res.Routes[j].RiskScore // lower risk first
	})
	if len(res.Routes) > 0 {
		res.Recommended = res.Routes[0].RouteID
	}
	return res
}

func routeWarnings(r Route, paths map[string]game.PathState) []string {
	var w []string
	for _, pid := range r.PathIDs {
		p := paths[pid]
		if p.Status == game.PathBlocked {
			w = append(w, "BLOCKED:"+pid)
		}
		if p.SurveillanceLevel >= 1 {
			w = append(w, "SURVEILLED:"+pid)
		}
	}
	return w
}
