package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/api"
	"github.com/ifx-run/ifx-launchpad-orchestrator/internal/config"
)

func main() {
	configPath := flag.String("config", "", "path to config.toml (or IFX_LAUNCHPAD_CONFIG)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	staticFS, err := fs.Sub(os.DirFS("public"), ".")
	if err != nil {
		log.Printf("warning: public/ not found, API-only mode: %v", err)
		staticFS = nil
	}

	srv := api.NewServer(cfg, staticFS)
	httpSrv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on http://%s (config: %s, debug=%v, jupiterTimeout=%ds, jupiterProxy=%q)",
			cfg.Addr(), cfg.SourcePath(), cfg.Server.Debug, cfg.Jupiter.TimeoutSeconds, cfg.Jupiter.ProxyURL)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown: %v\n", err)
	}
}
