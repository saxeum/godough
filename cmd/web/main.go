package main

import (
	"log/slog"
	"net/http"
	"os"
)

type application struct {
	logger *slog.Logger
}

func main() {
	addr := os.Getenv("SNIPPET_ADDR")
	if addr == "" {
		addr = ":4000"
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	app := &application{
		logger: logger,
	}

	logger.Info("starting server: http://localhost:4000")
	err := http.ListenAndServe(addr, app.routes(os.DirFS("ui/dist")))
	logger.Error(err.Error())
	os.Exit(1)
}
