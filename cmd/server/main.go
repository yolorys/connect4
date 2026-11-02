package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connect4/internal/httpapi"
	"connect4/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil { logger.Error("server stopped", "error", err); os.Exit(1) }
}

func run(logger *slog.Logger) error {
	// The container health check uses the application binary, so the final image
	// needs neither a shell nor curl.
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		client := &http.Client{Timeout: 3*time.Second}
		response, err := client.Get("http://127.0.0.1:8080/readyz")
		if err != nil { return err }
		defer response.Body.Close()
		if response.StatusCode != 200 { return fmt.Errorf("not ready: %s", response.Status) }
		return nil
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" { return errors.New("DATABASE_URL is required") }
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := store.Open(ctx, url)
	if err != nil { return err }
	defer db.Close()
	startup, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for {
		pingCtx, pingCancel := context.WithTimeout(startup, 2*time.Second)
		err = db.Ping(pingCtx)
		pingCancel()
		if err == nil { break }
		select {
		case <-startup.Done(): return fmt.Errorf("waiting for PostgreSQL: %w", startup.Err())
		case <-time.After(time.Second):
		}
	}
	if err := db.Migrate(startup); err != nil { return fmt.Errorf("database migration: %w", err) }
	address := os.Getenv("ADDR")
	if address == "" { address = ":8080" }
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" { staticDir = "web/dist" }
	server := &http.Server{
		Addr: address, Handler: httpapi.Handler(db, staticDir, logger),
		ReadHeaderTimeout: 5*time.Second, ReadTimeout: 15*time.Second,
		WriteTimeout: 15*time.Second, IdleTimeout: 60*time.Second,
	}
	result := make(chan error, 1)
	go func() { logger.Info("listening", "address", address); result <- server.ListenAndServe() }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) { return nil }; return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
