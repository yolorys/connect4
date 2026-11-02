package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connect4/internal/game"
)

const testID = "0123456789abcdef0123456789abcdef"

type fakeStore struct { state game.State; calls int; pingErr error; moveErr error }
func (f *fakeStore) Create(context.Context) (game.State, error) { f.state = game.New(testID); return f.state, nil }
func (f *fakeStore) Get(_ context.Context, id string) (game.State, error) {
	if id != f.state.ID { return game.State{}, game.ErrNotFound }; return f.state, nil
}
func (f *fakeStore) Move(_ context.Context, id string, col, expected int) (game.State, error) {
	f.calls++
	if f.moveErr != nil { return game.State{}, f.moveErr }
	if id != f.state.ID { return game.State{}, game.ErrNotFound }
	if expected != f.state.MoveCount { return game.State{}, game.ErrStale }
	err := f.state.Drop(col); return f.state, err
}
func (f *fakeStore) Ping(context.Context) error { return f.pingErr }

func testHandler(t *testing.T, store *fakeStore) http.Handler {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>Connect Four</title>"), 0600); err != nil { t.Fatal(err) }
	return Handler(store, dir, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func send(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder(); h.ServeHTTP(w, r); return w
}

func TestGameAPI(t *testing.T) {
	f := &fakeStore{}
	h := testHandler(t, f)
	w := send(h, "POST", "/api/games", "")
	if w.Code != 201 || w.Header().Get("Location") != "/api/games/"+testID { t.Fatalf("create: %d %s", w.Code, w.Body) }
	var created game.State
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil { t.Fatal(err) }
	if created.ID != testID || created.NextPlayer != 1 { t.Fatalf("bad create: %+v", created) }
	w = send(h, "POST", "/api/games/"+testID+"/moves", `{"column":3,"expectedMoveCount":0}`)
	if w.Code != 200 { t.Fatalf("move: %d %s", w.Code, w.Body) }
	w = send(h, "GET", "/api/games/"+testID, "")
	var state game.State
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil { t.Fatal(err) }
	if w.Code != 200 || state.MoveCount != 1 || state.Board[5][3] != 1 || state.LastMove == nil || state.LastMove.Column != 3 { t.Fatalf("get: %d %+v", w.Code, state) }
	w = send(h, "POST", "/api/games/"+testID+"/moves", `{"column":3,"expectedMoveCount":0}`)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "stale_state") || f.state.MoveCount != 1 { t.Fatalf("stale: %d %s", w.Code, w.Body) }
}

func TestMalformedRequestsNeverReachMoveStore(t *testing.T) {
	f := &fakeStore{state: game.New(testID)}
	h := testHandler(t, f)
	for _, body := range []string{``, `null`, `{}`, `{"column":1}`, `{"expectedMoveCount":0}`, `{"column":null,"expectedMoveCount":0}`, `{"column":1.5,"expectedMoveCount":0}`, `{"column":"1","expectedMoveCount":0}`, `{"column":7,"expectedMoveCount":0}`, `{"column":-1,"expectedMoveCount":0}`, `{"column":1,"expectedMoveCount":43}`, `{"column":1,"expectedMoveCount":-1}`, `{"column":1,"expectedMoveCount":0,"player":2}`, `{"column":1,"expectedMoveCount":0} {}`, strings.Repeat("x", 2000)} {
		w := send(h, "POST", "/api/games/"+testID+"/moves", body)
		if w.Code != 400 { t.Errorf("%q: status %d", body, w.Code) }
	}
	if f.calls != 0 { t.Fatalf("invalid requests reached store %d times", f.calls) }
	r := httptest.NewRequest("POST", "/api/games/"+testID+"/moves", strings.NewReader(`{"column":1,"expectedMoveCount":0}`))
	w := httptest.NewRecorder(); h.ServeHTTP(w, r)
	if w.Code != 415 { t.Fatalf("missing content type: %d", w.Code) }
}

func TestErrorsHealthAndStaticFiles(t *testing.T) {
	f := &fakeStore{state: game.New(testID)}
	h := testHandler(t, f)
	for _, path := range []string{"/api/games/invalid", "/api/games/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "/api/unknown", "/missing.js"} {
		if w := send(h, "GET", path, ""); w.Code != 404 { t.Errorf("%s: %d", path, w.Code) }
	}
	for _, path := range []string{"/healthz", "/readyz", "/"} {
		if w := send(h, "GET", path, ""); w.Code != 200 { t.Errorf("%s: %d", path, w.Code) }
	}
	f.pingErr = errors.New("offline")
	if w := send(h, "GET", "/readyz", ""); w.Code != 503 { t.Fatalf("readiness: %d", w.Code) }
	if w := send(h, "GET", "/healthz", ""); w.Code != 200 { t.Fatalf("liveness should not depend on database: %d", w.Code) }
	for _, tt := range []struct { err error; status int; code string }{
		{game.ErrFull, 409, "column_full"}, {game.ErrFinished, 409, "game_finished"},
		{game.ErrColumn, 400, "invalid_column"}, {game.ErrNotFound, 404, "not_found"},
		{errors.New("secret connection details"), 503, "internal_error"},
	} {
		f.moveErr = tt.err
		w := send(h, "POST", "/api/games/"+testID+"/moves", `{"column":0,"expectedMoveCount":0}`)
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.code) || strings.Contains(w.Body.String(), "secret") { t.Errorf("%v: %d %s", tt.err, w.Code, w.Body) }
		if w.Header().Get("Cache-Control") != "no-store" { t.Fatal("API response can be cached") }
	}
}
