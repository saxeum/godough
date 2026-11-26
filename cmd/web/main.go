package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/saxeum/godough/internal/app"
	"github.com/saxeum/godough/internal/config"
)

// bootTimeout bounds how long startup may spend reaching the database. A DSN
// that points at a host which drops packets would otherwise block for the
// operating system's TCP timeout, with the pod neither started nor crashed.
const bootTimeout = 10 * time.Second

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// The signal context is built here and passed in, so the shutdown path can be
	// driven by a test that cancels a context instead of signalling the test
	// process. Sending a real signal to your own process is a test that can take
	// the whole suite with it.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// run owns every resource, so its defers run on the way out.
	if err := run(ctx, logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// run is everything main does once the signals are wired: read the environment,
// reach the database, serve until ctx ends, and release. Its defers matter — the
// pool is closed on the way out of here, after the server has drained.
func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}

	bootCtx, bootDone := context.WithTimeout(ctx, bootTimeout)
	application, err := app.New(bootCtx, cfg, logger)
	bootDone()
	if err != nil {
		return err
	}
	defer application.Close()

	logger.Info("starting server", "addr", cfg.Addr)
	return application.Run(ctx)
}
