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

	// run owns every resource, so its defers run on the way out.
	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
