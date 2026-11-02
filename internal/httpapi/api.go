package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"connect4/internal/game"
)

type Store interface {
	Create(context.Context) (game.State, error)
	Get(context.Context, string) (game.State, error)
	Move(context.Context, string, int, int) (game.State, error)
	Ping(context.Context) error
}

var gameID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func Handler(store Store, staticDir string, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	respond := func(w http.ResponseWriter, state game.State, err error, status int) {
		if err == nil { writeJSON(w, status, state); return }
		code, message := "internal_error", "The server could not complete the request. Try reloading the game."
		switch {
		case errors.Is(err, game.ErrNotFound): status, code, message = 404, "not_found", err.Error()
		case errors.Is(err, game.ErrColumn): status, code, message = 400, "invalid_column", err.Error()
		case errors.Is(err, game.ErrFull): status, code, message = 409, "column_full", err.Error()
		case errors.Is(err, game.ErrFinished): status, code, message = 409, "game_finished", err.Error()
		case errors.Is(err, game.ErrStale): status, code, message = 409, "stale_state", err.Error()
		default: status = 503; logger.Error("request failed", "error", err)
		}
		writeJSON(w, status, map[string]string{"code": code, "error": message})
	}
	validID := func(w http.ResponseWriter, r *http.Request) bool {
		if !gameID.MatchString(r.PathValue("id")) {
			writeJSON(w, 404, map[string]string{"code": "not_found", "error": "game not found"})
			return false
		}
		return true
	}
	mux.HandleFunc("POST /api/games", func(w http.ResponseWriter, r *http.Request) {
		state, err := store.Create(r.Context())
		if err == nil { w.Header().Set("Location", "/api/games/" + state.ID) }
		respond(w, state, err, http.StatusCreated)
	})
	mux.HandleFunc("GET /api/games/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !validID(w, r) { return }
		state, err := store.Get(r.Context(), r.PathValue("id"))
		respond(w, state, err, http.StatusOK)
	})
	mux.HandleFunc("POST /api/games/{id}/moves", func(w http.ResponseWriter, r *http.Request) {
		if !validID(w, r) { return }
		mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mediaType != "application/json" {
			writeJSON(w, 415, map[string]string{"code": "invalid_request", "error": "Content-Type must be application/json"}); return
		}
		var request struct {
			Column *int `json:"column"`
			ExpectedMoveCount *int `json:"expectedMoveCount"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		err := decoder.Decode(&request)
		if err != nil || request.Column == nil || request.ExpectedMoveCount == nil || decoder.Decode(new(any)) != io.EOF {
			writeJSON(w, 400, map[string]string{"code": "invalid_request", "error": "Provide column and expectedMoveCount as integers in one JSON object."}); return
		}
		if *request.Column < 0 || *request.Column >= game.Columns || *request.ExpectedMoveCount < 0 || *request.ExpectedMoveCount > 42 {
			writeJSON(w, 400, map[string]string{"code": "invalid_request", "error": "column must be 0–6 and expectedMoveCount must be 0–42."}); return
		}
		state, err := store.Move(r.Context(), r.PathValue("id"), *request.Column, *request.ExpectedMoveCount)
		respond(w, state, err, http.StatusOK)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := store.Ping(ctx); err != nil { writeJSON(w, 503, map[string]string{"status": "unavailable"}); return }
		writeJSON(w, 200, map[string]string{"status": "ready"})
	})
	for _, method := range []string{"GET", "POST"} {
		mux.HandleFunc(method+" /api/", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 404, map[string]string{"code": "not_found", "error": "API endpoint not found"}) })
	}
	files := http.FileServer(http.Dir(staticDir))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// This app has no client-side routes: absent assets must really be 404s.
		if r.URL.Path == "/" {
			if _, err := os.Stat(filepath.Join(staticDir, "index.html")); err != nil {
				http.Error(w, "Frontend is not built. Run npm --prefix web run build.", 503); return
			}
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
