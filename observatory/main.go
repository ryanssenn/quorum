package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ryansenn/ryanDB/internal/harness"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	port := flag.Int("port", 8080, "Observatory UI port")
	noBrowser := flag.Bool("no-browser", false, "skip opening browser")
	binary := flag.String("binary", "", "path to ryanDB binary")
	demoPace := flag.Bool("demo", true, "compress scenario waits")
	composeUp := flag.Bool("compose-up", false, "start Prometheus and Grafana via docker compose")
	flag.Parse()

	repoRoot := findRepoRoot()
	binaryPath, err := ensureBinary(repoRoot, *binary)
	if err != nil {
		log.Fatalf("binary: %v", err)
	}

	if *composeUp {
		if err := startCompose(repoRoot); err != nil {
			log.Printf("warning: %v", err)
		} else {
			log.Println("monitoring stack started (Prometheus :9090, Grafana :3000)")
		}
	}

	srv := NewServer(binaryPath, repoRoot)

	if flag.NArg() >= 1 {
		scenario, err := LoadScenario(flag.Arg(0))
		if err != nil {
			log.Fatalf("load scenario: %v", err)
		}
		srv.mu.Lock()
		srv.scenario = scenario
		srv.demoPace = *demoPace
		if scenario.Showcase {
			srv.demoPace = false
		}
		srv.mu.Unlock()

		harness.KillPorts(scenario.Nodes)
		srv.mu.Lock()
		srv.cluster = NewCluster(scenario.Nodes)
		srv.mu.Unlock()
		_ = writePrometheusTargets(repoRoot, scenario.Nodes)

		log.Printf("starting %d-node cluster for scenario...", scenario.Nodes)
		if scenario.Showcase {
			srv.mu.Lock()
			srv.showcaseStart = time.Now()
			srv.mu.Unlock()
			if err := srv.cluster.StartStaggered(binaryPath, true, 500*time.Millisecond); err != nil {
				log.Fatalf("start cluster: %v", err)
			}
		} else {
			if err := srv.cluster.StartAll(binaryPath, true); err != nil {
				log.Fatalf("start cluster: %v", err)
			}
		}
		srv.mu.Lock()
		srv.clusterStarted = true
		srv.appendLog("cluster started (" + fmt.Sprintf("%d", scenario.Nodes) + " nodes)")
		srv.appendLog("running scenario: " + scenario.Name)
		srv.mu.Unlock()
		go srv.runScenario()
	} else {
		log.Printf("observatory ready; open UI to configure cluster and run scenarios")
		srv.appendLog("observatory ready")
	}

	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("static files: %v", err)
	}

	mux := http.NewServeMux()
	srv.registerRoutes(mux, http.FileServer(http.FS(static)))

	addr := fmt.Sprintf(":%d", *port)
	url := "http://localhost" + addr

	go func() {
		log.Printf("observatory at %s", url)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	if !*noBrowser {
		openBrowser(url)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("shutting down...")
	srv.cluster.StopAll()
}
