package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/TiaraBasori/PaperValet/internal/app"
	"github.com/TiaraBasori/PaperValet/internal/config"
)

var (
	version   = "0.1.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	configPath := flag.String("config", "config.json", "path to config.json")
	showVersion := flag.Bool("version", false, "show version info")
	flag.Parse()

	if *showVersion {
		fmt.Printf("PaperValet %s\n", version)
		fmt.Printf("  build: %s\n", buildTime)
		fmt.Printf("  commit: %s\n", gitCommit)
		fmt.Printf("  go: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ load config: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: cp config.example.json config.json && edit api_id/api_hash\n")
		os.Exit(1)
	}

	application, err := app.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ init: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = application.Shutdown(shutdownCtx)
	}()

	if err := application.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "❌ run: %v\n", err)
		os.Exit(1)
	}
}