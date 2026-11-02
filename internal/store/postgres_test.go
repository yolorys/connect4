package store

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"connect4/internal/game"
)

func integrationStore(t *testing.T) (*Postgres, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" { t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests") }
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	s, err := Open(ctx, url)
	if err != nil { t.Fatal(err) }
	t.Cleanup(s.Close)
	if err := s.Ping(ctx); err != nil { t.Fatal(err) }
	if err := s.Migrate(ctx); err != nil { t.Fatal(err) }
	if err := s.Migrate(ctx); err != nil { t.Fatalf("second migration: %v", err) }
	return s, ctx
}

func newTestGame(t *testing.T, s *Postgres, ctx context.Context) game.State {
	t.Helper()
	state, err := s.Create(ctx)
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second); defer cancel()
		if _, err := s.pool.Exec(cleanup, `DELETE FROM games WHERE id=$1`, state.ID); err != nil { t.Errorf("cleanup game: %v", err) }
	})
	return state
}

func TestPostgresPersistenceAndRejectedMoves(t *testing.T) {
	s, ctx := integrationStore(t)
	state := newTestGame(t, s, ctx)
	for i := 0; i < 6; i++ {
		var err error
		state, err = s.Move(ctx, state.ID, 0, i)
		if err != nil { t.Fatal(err) }
	}
	if _, err := s.Move(ctx, state.ID, 0, 6); !errors.Is(err, game.ErrFull) { t.Fatalf("full column: %v", err) }
	if _, err := s.Move(ctx, state.ID, 2, 5); !errors.Is(err, game.ErrStale) { t.Fatalf("stale: %v", err) }
	if _, err := s.Move(ctx, state.ID, -1, 6); !errors.Is(err, game.ErrColumn) { t.Fatalf("invalid column: %v", err) }
	// A separate connection pool represents a restarted application.
	other, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil { t.Fatal(err) }; defer other.Close()
	restored, err := other.Get(ctx, state.ID)
	if err != nil || !reflect.DeepEqual(restored, state) { t.Fatalf("restore: %+v %v", restored, err) }
	if _, err := s.Get(ctx, "00000000000000000000000000000000"); !errors.Is(err, game.ErrNotFound) { t.Fatalf("missing game: %v", err) }
	if _, err := s.Move(ctx, "00000000000000000000000000000000", 0, 0); !errors.Is(err, game.ErrNotFound) { t.Fatalf("missing move: %v", err) }
}

func TestPostgresSerializesConcurrentMoves(t *testing.T) {
	s, ctx := integrationStore(t)
	state := newTestGame(t, s, ctx)
	start := make(chan struct{})
	results := make(chan error, 10)
	var group sync.WaitGroup
	for i := 0; i < 10; i++ {
		group.Add(1)
		go func() { defer group.Done(); <-start; _, err := s.Move(ctx, state.ID, 3, 0); results <- err }()
	}
	close(start); group.Wait(); close(results)
	accepted, stale := 0, 0
	for err := range results {
		if err == nil { accepted++ } else if errors.Is(err, game.ErrStale) { stale++ } else { t.Fatal(err) }
	}
	if accepted != 1 || stale != 9 { t.Fatalf("accepted=%d stale=%d", accepted, stale) }
	restored, err := s.Get(ctx, state.ID)
	if err != nil || restored.MoveCount != 1 || restored.NextPlayer != 2 { t.Fatalf("concurrent result: %+v %v", restored, err) }
}

func TestPostgresRejectsMovesAfterWin(t *testing.T) {
	s, ctx := integrationStore(t)
	state := newTestGame(t, s, ctx)
	for i, column := range []int{0,1,0,1,0,1,0} {
		var err error
		state, err = s.Move(ctx, state.ID, column, i)
		if err != nil { t.Fatal(err) }
	}
	if _, err := s.Move(ctx, state.ID, 2, 7); !errors.Is(err, game.ErrFinished) { t.Fatalf("finished: %v", err) }
	got, err := s.Get(ctx, state.ID)
	if err != nil || !reflect.DeepEqual(got, state) { t.Fatalf("changed finished game: %+v %v", got, err) }
}
