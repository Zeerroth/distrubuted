package game

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"os"
)

// Terrain and special-role string constants (spec §2.1).
const (
	TerrainPlains    = "PLAINS"
	TerrainMountains = "MOUNTAINS"
	TerrainForest    = "FOREST"
	TerrainFortress  = "FORTRESS"
	TerrainVolcanic  = "VOLCANIC"
	TerrainSwamp     = "SWAMP"
)

// Region is static map data (spec §2.1).
type Region struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Terrain      string `json:"terrain"`
	SpecialRole  string `json:"specialRole"`
	StartControl string `json:"startControl"`
	StartThreat  int    `json:"startThreat"`
}

// Path is static map data (spec §2.2). All paths are bidirectional. cost = turns
// required to traverse.
type Path struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Cost int    `json:"cost"`
}

type edge struct {
	to     string
	cost   int
	pathID string
}

// GameMap is the immutable graph: 22 regions + the paths from the spec table.
type GameMap struct {
	Regions map[string]Region
	Paths   map[string]Path
	adj     map[string][]edge // region id -> outgoing edges (both directions added)
}

type rawMapFile struct {
	Regions []Region `json:"regions"`
	Paths   []Path   `json:"paths"`
}

// LoadMap reads config/map.json and builds the bidirectional adjacency.
func LoadMap(mapPath string) (*GameMap, error) {
	b, err := os.ReadFile(mapPath)
	if err != nil {
		return nil, fmt.Errorf("read map %q: %w", mapPath, err)
	}
	var raw rawMapFile
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse map: %w", err)
	}
	return NewMap(raw.Regions, raw.Paths)
}

// NewMap builds a GameMap from region and path slices (used by tests too).
func NewMap(regions []Region, paths []Path) (*GameMap, error) {
	m := &GameMap{
		Regions: make(map[string]Region, len(regions)),
		Paths:   make(map[string]Path, len(paths)),
		adj:     make(map[string][]edge),
	}
	for _, r := range regions {
		m.Regions[r.ID] = r
	}
	for _, p := range paths {
		if _, ok := m.Regions[p.From]; !ok {
			return nil, fmt.Errorf("path %q references unknown region %q", p.ID, p.From)
		}
		if _, ok := m.Regions[p.To]; !ok {
			return nil, fmt.Errorf("path %q references unknown region %q", p.ID, p.To)
		}
		m.Paths[p.ID] = p
		// bidirectional
		m.adj[p.From] = append(m.adj[p.From], edge{to: p.To, cost: p.Cost, pathID: p.ID})
		m.adj[p.To] = append(m.adj[p.To], edge{to: p.From, cost: p.Cost, pathID: p.ID})
	}
	return m, nil
}

// PathBetween returns the pathID directly connecting two adjacent regions, or "".
func (m *GameMap) PathBetween(a, b string) (string, bool) {
	for _, e := range m.adj[a] {
		if e.to == b {
			return e.pathID, true
		}
	}
	return "", false
}

// Hops returns the minimum number of edges between a and b (unweighted BFS).
// This is graph.distance from the DETECTION formula (spec §3.6): a Nazgul with
// detectionRange R detects the Ring Bearer when Hops(nazgul, rb) <= R. Returns
// a large sentinel if unreachable.
func (m *GameMap) Hops(a, b string) int {
	if a == b {
		return 0
	}
	const inf = 1 << 30
	dist := map[string]int{a: 0}
	queue := []string{a}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range m.adj[cur] {
			if _, seen := dist[e.to]; seen {
				continue
			}
			dist[e.to] = dist[cur] + 1
			if e.to == b {
				return dist[e.to]
			}
			queue = append(queue, e.to)
		}
	}
	return inf
}

// ShortestCost returns the minimum total traversal COST (turns) between a and b
// using Dijkstra over path costs. This is "turns to reach" used by the analysis
// pipelines (spec §32–33). Returns a large sentinel if unreachable.
func (m *GameMap) ShortestCost(a, b string) int {
	const inf = 1 << 30
	dist := map[string]int{a: 0}
	pq := &minHeap{{node: a, dist: 0}}
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(hnode)
		if cur.node == b {
			return cur.dist
		}
		if d, ok := dist[cur.node]; ok && cur.dist > d {
			continue
		}
		for _, e := range m.adj[cur.node] {
			nd := cur.dist + e.cost
			if old, ok := dist[e.to]; !ok || nd < old {
				dist[e.to] = nd
				heap.Push(pq, hnode{node: e.to, dist: nd})
			}
		}
	}
	if d, ok := dist[b]; ok {
		return d
	}
	return inf
}

// Endpoints returns the two region ids of a path.
func (m *GameMap) Endpoints(pathID string) (string, string, bool) {
	p, ok := m.Paths[pathID]
	if !ok {
		return "", "", false
	}
	return p.From, p.To, true
}

// --- tiny binary heap for Dijkstra (no external deps) ---

type hnode struct {
	node string
	dist int
}

type minHeap []hnode

func (h minHeap) Len() int            { return len(h) }
func (h minHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h minHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x interface{}) { *h = append(*h, x.(hnode)) }
func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
