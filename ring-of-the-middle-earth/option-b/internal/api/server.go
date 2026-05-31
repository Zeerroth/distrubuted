package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"rotr/internal/bus"
	"rotr/internal/config"
	"rotr/internal/engine"
	"rotr/internal/game"
	"rotr/internal/pipeline"
	"rotr/internal/router"
)

// Server wires the HTTP API (spec §34) to the engine, bus and hub.
type Server struct {
	cfg  *config.GameConfig
	gmap *game.GameMap
	eng  *engine.Engine
	bus  bus.Bus
	hub  *Hub
}

func NewServer(cfg *config.GameConfig, gmap *game.GameMap, eng *engine.Engine, b bus.Bus, hub *Hub) *Server {
	return &Server{cfg: cfg, gmap: gmap, eng: eng, bus: b, hub: hub}
}

// Routes registers all endpoints from spec §34 (plus demo conveniences: reset,
// config, and static UI serving) and wraps everything in permissive CORS so the
// browser UI can drive the engine — including cross-origin from a second engine
// instance in the docker setup (Light=:8081 talking to Dark=:8082).
func (s *Server) Routes(uiDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/game/start", s.start)
	mux.HandleFunc("/game/tick", s.tick)   // demo: advance one turn on demand
	mux.HandleFunc("/game/reset", s.reset) // demo: re-seed the game between takes
	mux.HandleFunc("/game/state", s.state)
	mux.HandleFunc("/order", s.order)
	mux.HandleFunc("/orders/available", s.ordersAvailable)
	mux.HandleFunc("/config/units", s.configUnits) // UI builds pickers from config
	mux.HandleFunc("/config/map", s.configMap)     // UI renders the map from config
	mux.HandleFunc("/analysis/routes", s.analysisRoutes)
	mux.HandleFunc("/analysis/intercept", s.analysisIntercept)
	mux.HandleFunc("/events", s.events)
	if uiDir != "" {
		// Serve the UI same-origin so http://localhost:8080/ just works.
		mux.Handle("/", http.FileServer(http.Dir(uiDir)))
	}
	return cors(mux)
}

// cors adds permissive CORS headers and answers preflight OPTIONS requests so the
// browser can POST /order with a JSON content-type.
func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// reset re-seeds the game to its initial state (great for re-shooting a scenario).
func (s *Server) reset(w http.ResponseWriter, r *http.Request) {
	s.eng.Reset()
	writeJSON(w, http.StatusOK, s.eng.Snapshot())
}

// configUnits returns the unit configs in load order so the UI can build
// side-grouped pickers and colour units — config-driven all the way to the UI.
func (s *Server) configUnits(w http.ResponseWriter, r *http.Request) {
	out := make([]config.UnitConfig, 0, len(s.cfg.UnitOrder))
	for _, id := range s.cfg.UnitOrder {
		out = append(out, s.cfg.Units[id])
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"units":           out,
		"hiddenUntilTurn": s.cfg.HiddenUntilTurn,
		"maxTurns":        s.cfg.MaxTurns,
	})
}

// configMap returns the static map (22 regions + paths) for the UI to draw.
func (s *Server) configMap(w http.ResponseWriter, r *http.Request) {
	regions := make([]game.Region, 0, len(s.gmap.Regions))
	for _, rg := range s.gmap.Regions {
		regions = append(regions, rg)
	}
	paths := make([]game.Path, 0, len(s.gmap.Paths))
	for _, p := range s.gmap.Paths {
		paths = append(paths, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"regions": regions, "paths": paths})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"mode": "HVH", "turn": s.eng.Snapshot().Turn})
}

// tick advances exactly one turn — convenient for the instructor-driven demo.
func (s *Server) tick(w http.ResponseWriter, r *http.Request) {
	snap := s.eng.ProcessTurn()
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	side := r.URL.Query().Get("side")
	snap := s.eng.Snapshot()
	if side == "dark" {
		// Dark Side never sees the bearer's region (spec §34 / §30).
		snap.RingBearerRegion = ""
		if u, ok := snap.Units[router.RingBearerUnitID]; ok {
			u.Region = ""
			snap.Units[router.RingBearerUnitID] = u
		}
	}
	writeJSON(w, http.StatusOK, snap)
}

// order publishes to game.orders.raw (returns 202) AND validates+applies via the
// engine. Invalid orders return the §5.4 error code and a DLQ record.
func (s *Server) order(w http.ResponseWriter, r *http.Request) {
	var o game.Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "BAD_JSON"})
		return
	}
	raw, _ := json.Marshal(o)
	_ = s.bus.Produce("game.orders.raw", o.PlayerID, raw)

	res := s.eng.Submit(o)
	if !res.Valid {
		dlq, _ := json.Marshal(map[string]any{"errorCode": res.ErrorCode, "order": o})
		_ = s.bus.Produce("game.dlq", res.ErrorCode, dlq)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"errorCode": res.ErrorCode})
		return
	}
	valid, _ := json.Marshal(o)
	_ = s.bus.Produce("game.orders.validated", o.UnitID, valid)
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "ACCEPTED"})
}

// ordersAvailable returns the legal order types for a unit (spec §5.1). Minimal
// implementation: derive from config class/side.
func (s *Server) ordersAvailable(w http.ResponseWriter, r *http.Request) {
	unitID := r.URL.Query().Get("unitId")
	c, ok := s.cfg.Units[unitID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "UNKNOWN_UNIT"})
		return
	}
	orders := []string{game.OrderAssignRoute, game.OrderRedirectUnit}
	switch {
	case c.Class == "RingBearer":
		orders = append(orders, game.OrderDestroyRing)
	case c.Maia:
		orders = append(orders, game.OrderMaiaAbility)
	case c.CanFortify:
		orders = append(orders, game.OrderFortifyRegion, game.OrderAttackRegion, game.OrderReinforceRegion)
	}
	if c.Side == config.SideShadow {
		orders = append(orders, game.OrderBlockPath, game.OrderSearchPath, game.OrderAttackRegion)
		if c.Class == "Nazgul" {
			orders = append(orders, game.OrderDeployNazgul)
		}
	} else if c.Class == "FellowshipGuard" || c.Class == "GondorArmy" {
		orders = append(orders, game.OrderBlockPath, game.OrderAttackRegion, game.OrderReinforceRegion)
	}
	writeJSON(w, http.StatusOK, map[string]any{"unitId": unitID, "available": orders})
}

func (s *Server) analysisRoutes(w http.ResponseWriter, r *http.Request) {
	snap := s.eng.Snapshot()
	var candidates []pipeline.Route
	for _, rc := range canonicalRoutes() {
		route, ok := pipeline.BuildRoute(rc.id, s.gmap, rc.start, rc.paths)
		if ok {
			candidates = append(candidates, route)
		}
	}
	res := pipeline.RunRouteRisk(context.Background(), candidates, s.gmap, snap.Paths, snap.Regions, s.nazgulRegions(snap))
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) analysisIntercept(w http.ResponseWriter, r *http.Request) {
	snap := s.eng.Snapshot()
	// Use the lowest-risk canonical route as the candidate the bearer is likely on.
	var cands []pipeline.InterceptCandidate
	for _, rc := range canonicalRoutes() {
		route, ok := pipeline.BuildRoute(rc.id, s.gmap, rc.start, rc.paths)
		if !ok {
			continue
		}
		// cumulative cost to reach each region along the route
		cum := 0
		cur := rc.start
		for _, pid := range rc.paths {
			p := s.gmap.Paths[pid]
			cum += p.Cost
			next := p.To
			if cur == p.To {
				next = p.From
			}
			cur = next
			for _, nazID := range s.nazgulIDs() {
				nazReg := snap.Units[nazID].Region
				if snap.Units[nazID].Status != game.StatusActive {
					continue
				}
				cands = append(cands, pipeline.InterceptCandidate{
					UnitID: nazID, NazgulRegion: nazReg, TargetRegion: next,
					RBTurnsToReach: cum, RouteLength: route.TotalCost,
				})
			}
		}
	}
	plan := pipeline.RunIntercept(context.Background(), cands, s.gmap)
	writeJSON(w, http.StatusOK, plan)
}

// events is the SSE stream (spec §34). ?side=light|dark selects the channel.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	side := r.URL.Query().Get("side")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch, unsub := s.hub.Subscribe(side)
	defer unsub()
	fmt.Fprintf(w, ": connected as %s\n\n", side)
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case b, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}

// ---- helpers ----

func (s *Server) nazgulIDs() []string {
	var out []string
	for _, id := range s.cfg.UnitOrder {
		if s.cfg.Units[id].Class == "Nazgul" {
			out = append(out, id)
		}
	}
	return out
}

func (s *Server) nazgulRegions(snap router.WorldStateSnapshot) []string {
	var out []string
	for _, id := range s.nazgulIDs() {
		if u, ok := snap.Units[id]; ok && u.Status == game.StatusActive {
			out = append(out, u.Region)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

type routeCandidate struct {
	id    string
	start string
	paths []string
}

// canonicalRoutes encodes the four Ring-Bearer routes (spec §2.3).
func canonicalRoutes() []routeCandidate {
	return []routeCandidate{
		{id: "route-1-fellowship", start: "the-shire", paths: []string{
			"shire-to-bree", "bree-to-weathertop", "weathertop-to-rivendell", "rivendell-to-moria",
			"moria-to-lothlorien", "lothlorien-to-emyn-muil", "emyn-muil-to-ithilien",
			"ithilien-to-cirith-ungol", "cirith-ungol-to-mount-doom"}},
		{id: "route-2-northern-bypass", start: "the-shire", paths: []string{
			"shire-to-bree", "bree-to-rivendell", "rivendell-to-lothlorien", "lothlorien-to-emyn-muil",
			"emyn-muil-to-dead-marshes", "dead-marshes-to-ithilien", "ithilien-to-cirith-ungol",
			"cirith-ungol-to-mount-doom"}},
		{id: "route-3-dark-route", start: "the-shire", paths: []string{
			"shire-to-bree", "bree-to-rivendell", "rivendell-to-lothlorien", "lothlorien-to-emyn-muil",
			"emyn-muil-to-dead-marshes", "dead-marshes-to-mordor", "mordor-to-mount-doom"}},
		{id: "route-4-southern-corridor", start: "the-shire", paths: []string{
			"shire-to-tharbad", "tharbad-to-fords-of-isen", "fords-of-isen-to-edoras", "edoras-to-minas-tirith",
			"minas-tirith-to-osgiliath", "osgiliath-to-minas-morgul", "minas-morgul-to-cirith-ungol",
			"cirith-ungol-to-mount-doom"}},
	}
}
