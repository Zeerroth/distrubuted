// Package pipeline implements the two analysis pipelines (spec §32 Route Risk,
// §33 Interception). Each exposes a PURE scoring function (unit-tested) plus a
// concurrent worker-pool runner (dispatcher -> buffered ch -> 4 workers ->
// aggregator) with context cancellation and a 2s timeout, per the spec.
package pipeline

import "rotr/internal/game"

// Route is a Ring-Bearer route candidate derived from an ordered path list.
type Route struct {
	ID        string
	PathIDs   []string
	Regions   []string // ordered regions visited, including the start region
	DestRegks []string // destination regions (every region entered; excludes start)
	TotalCost int      // sum of path costs == turns to traverse the whole route
}

// BuildRoute walks an ordered list of path ids from a start region and resolves
// the regions visited and the total cost. Returns ok=false if a path does not
// connect to the current region (an ill-formed route).
func BuildRoute(id string, m *game.GameMap, start string, pathIDs []string) (Route, bool) {
	r := Route{ID: id, PathIDs: pathIDs, Regions: []string{start}}
	cur := start
	for _, pid := range pathIDs {
		p, ok := m.Paths[pid]
		if !ok {
			return Route{}, false
		}
		var next string
		switch cur {
		case p.From:
			next = p.To
		case p.To:
			next = p.From
		default:
			return Route{}, false // path not incident to current region
		}
		r.Regions = append(r.Regions, next)
		r.DestRegks = append(r.DestRegks, next)
		r.TotalCost += p.Cost
		cur = next
	}
	return r, true
}
