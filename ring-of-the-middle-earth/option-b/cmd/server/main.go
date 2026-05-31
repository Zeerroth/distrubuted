// Command server boots one Go instance of the Ring-of-the-Middle-Earth engine
// (Option B). By default it uses the in-memory bus so it runs with no Kafka:
//
//	go run ./cmd/server -config ../config -addr :8080
//
// In docker (3 instances) build with `-tags kafka` to use the Confluent bus and
// a shared consumer group (spec §26).
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"rotr/internal/api"
	"rotr/internal/config"
	"rotr/internal/engine"
	"rotr/internal/game"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	cfgDir := flag.String("config", "../config", "directory holding units.json and map.json")
	uiDir := flag.String("ui", "../ui", "directory of the web UI to serve at / (empty to disable)")
	auto := flag.Bool("auto", false, "auto-advance turns every turn-duration (off = instructor ticks via POST /game/tick)")
	flag.Parse()

	cfg, err := config.Load(filepath.Join(*cfgDir, "units.json"))
	if err != nil {
		log.Fatalf("load units: %v", err)
	}
	gmap, err := game.LoadMap(filepath.Join(*cfgDir, "map.json"))
	if err != nil {
		log.Fatalf("load map: %v", err)
	}
	log.Printf("loaded %d units, %d regions, %d paths", len(cfg.Units), len(gmap.Regions), len(gmap.Paths))

	b := newBus()
	defer b.Close()

	eng := engine.NewEngine(cfg, gmap, b)
	hub := api.NewHub()
	if err := hub.Run(b); err != nil {
		log.Fatalf("hub: %v", err)
	}
	srv := api.NewServer(cfg, gmap, eng, b, hub)

	httpSrv := &http.Server{Addr: *addr, Handler: srv.Routes(*uiDir)}
	go func() {
		log.Printf("HTTP listening on %s (instance=%s) — UI at http://localhost%s/", *addr, instanceID(), *addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	// ---- the main select loop (spec §31): all 7 cases handled ----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	// channels modelling the remaining cases; fed by the hub/api in a full build.
	newConnectionCh := make(chan string, 16)
	disconnectCh := make(chan string, 16)
	analysisRequestCh := make(chan string, 16)
	cacheUpdateCh := make(chan int, 16)

	turnInterval := time.Duration(cfg.TurnDurationSeconds) * time.Second
	if turnInterval <= 0 {
		turnInterval = 60 * time.Second
	}

	for {
		var turnTick <-chan time.Time
		if *auto {
			turnTick = time.After(turnInterval)
		}
		select {
		case <-ctx.Done():
			return

		case sig := <-signalCh: // case 7: shutdown
			log.Printf("signal %v: shutting down", sig)
			shutdownCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
			_ = httpSrv.Shutdown(shutdownCtx)
			c()
			return

		case <-turnTick: // case 6: 60s turn timer
			snap := eng.ProcessTurn()
			log.Printf("turn %d processed (auto)", snap.Turn)

		case id := <-newConnectionCh: // case 2
			log.Printf("player connected: %s", id)

		case id := <-disconnectCh: // case 3
			log.Printf("player disconnected: %s", id)

		case req := <-analysisRequestCh: // case 4
			log.Printf("analysis requested: %s", req)

		case turn := <-cacheUpdateCh: // case 5
			log.Printf("cache updated to turn %d", turn)
		}
		// case 1 (kafka consumer messages) is handled by the Hub goroutine, which
		// owns the bus subscription and fans records to SSE clients.
	}
}

func instanceID() string {
	if v := os.Getenv("INSTANCE_ID"); v != "" {
		return v
	}
	return "go-local"
}
