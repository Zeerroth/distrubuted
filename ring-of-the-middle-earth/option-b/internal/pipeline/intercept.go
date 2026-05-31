package pipeline

import (
	"context"
	"sync"
	"time"

	"rotr/internal/game"
)

// InterceptScore is the PURE interception function (spec §22.1 / §33), scoring
// one (Nazgul, route-region) pair:
//
//	turnsToIntercept = shortestPath(nazgul.region, routeRegion)   [in turns/cost]
//	interceptWindow  = rbTurnsToReach - turnsToIntercept
//	score = interceptWindow >= 0 ? 1.0 - (turnsToIntercept / routeLength) : 0.0
//
// A non-negative window means the Nazgul can arrive at or before the Ring Bearer.
func InterceptScore(turnsToIntercept, rbTurnsToReach, routeLength int) float64 {
	window := rbTurnsToReach - turnsToIntercept
	if window < 0 || routeLength <= 0 {
		return 0.0
	}
	return 1.0 - float64(turnsToIntercept)/float64(routeLength)
}

// InterceptCandidate is one (Nazgul, route-region) job for a worker.
type InterceptCandidate struct {
	UnitID         string
	NazgulRegion   string
	TargetRegion   string
	RBTurnsToReach int // ring bearer's cost to reach TargetRegion along the route
	RouteLength    int // total route cost (denominator)
}

// InterceptEntry is one scored plan entry (spec §33 byUnit element).
type InterceptEntry struct {
	UnitID       string  `json:"unitId"`
	TargetRegion string  `json:"targetRegion"`
	Score        float64 `json:"score"`
}

// InterceptPlan is the Pipeline 2 output (spec §33).
type InterceptPlan struct {
	ByUnit  []InterceptEntry `json:"byUnit"`
	Partial bool             `json:"partial"`
}

// RunIntercept is the concurrent runner (spec §33): dispatcher -> buffered ch
// (cap 30) -> 4 workers -> aggregator, keeping the best-scoring target region
// per Nazgul. Honors context cancellation and a 2s timeout.
func RunIntercept(ctx context.Context, candidates []InterceptCandidate, m *game.GameMap) InterceptPlan {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	const workers = 4
	in := make(chan InterceptCandidate, 30)
	out := make(chan InterceptEntry)

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

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for c := range in {
				tti := m.ShortestCost(c.NazgulRegion, c.TargetRegion)
				score := InterceptScore(tti, c.RBTurnsToReach, c.RouteLength)
				select {
				case out <- InterceptEntry{UnitID: c.UnitID, TargetRegion: c.TargetRegion, Score: score}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() { wg.Wait(); close(out) }()

	best := map[string]InterceptEntry{}
	plan := InterceptPlan{}
	for {
		select {
		case e, ok := <-out:
			if !ok {
				goto done
			}
			if cur, seen := best[e.UnitID]; !seen || e.Score > cur.Score {
				best[e.UnitID] = e
			}
		case <-ctx.Done():
			plan.Partial = true
			goto done
		}
	}
done:
	for _, e := range best {
		plan.ByUnit = append(plan.ByUnit, e)
	}
	return plan
}
