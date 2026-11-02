// Package store persists ordered moves. The game package remains the authority
// for gravity, turns, legal moves, and outcomes.
package store

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"connect4/internal/game"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Postgres struct { pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil { return nil, err }
	config.MaxConns = 10
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil { return nil, err }
	return &Postgres{pool: pool}, nil
}

func (s *Postgres) Close() { s.pool.Close() }
func (s *Postgres) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Migrate applies versioned SQL files once, in filename order. The advisory lock
// also makes concurrent application startups safe.
func (s *Postgres) Migrate(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(431004)`); err != nil { return err }
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil { return err }
	entries, err := migrations.ReadDir("migrations")
	if err != nil { return err }
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") { continue }
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, entry.Name()).Scan(&exists); err != nil { return err }
		if exists { continue }
		sql, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil { return err }
		if _, err = tx.Exec(ctx, string(sql)); err != nil { return fmt.Errorf("migration %s: %w", entry.Name(), err) }
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, entry.Name()); err != nil { return err }
	}
	return tx.Commit(ctx)
}

func (s *Postgres) Create(ctx context.Context) (game.State, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil { return game.State{}, err }
	id := hex.EncodeToString(bytes[:])
	if _, err := s.pool.Exec(ctx, `INSERT INTO games(id) VALUES ($1)`, id); err != nil { return game.State{}, err }
	return game.New(id), nil
}

func (s *Postgres) Get(ctx context.Context, id string) (game.State, error) {
	// One query gives a consistent snapshot of the game and all its moves.
	var columns []int
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(
		array_agg(m.column_index ORDER BY m.move_number) FILTER (WHERE m.move_number IS NOT NULL),
		'{}'::integer[]) FROM games g LEFT JOIN moves m ON m.game_id=g.id
		WHERE g.id=$1 GROUP BY g.id`, id).Scan(&columns)
	if errors.Is(err, pgx.ErrNoRows) { return game.State{}, game.ErrNotFound }
	if err != nil { return game.State{}, err }
	return reconstruct(id, columns)
}

func (s *Postgres) Move(ctx context.Context, id string, column, expected int) (game.State, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil { return game.State{}, err }
	defer tx.Rollback(ctx)

	// Always lock the parent BEFORE reading moves. A waiting request then sees
	// the previous transaction's committed moves in its next query.
	var lockedID string
	err = tx.QueryRow(ctx, `SELECT id FROM games WHERE id=$1 FOR UPDATE`, id).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) { return game.State{}, game.ErrNotFound }
	if err != nil { return game.State{}, err }
	rows, err := tx.Query(ctx, `SELECT column_index FROM moves WHERE game_id=$1 ORDER BY move_number`, id)
	if err != nil { return game.State{}, err }
	columns, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil { return game.State{}, err }
	state, err := reconstruct(id, columns)
	if err != nil { return game.State{}, err }
	if state.MoveCount != expected { return game.State{}, game.ErrStale }
	if err := state.Drop(column); err != nil { return game.State{}, err }
	if _, err = tx.Exec(ctx, `INSERT INTO moves(game_id,move_number,column_index) VALUES ($1,$2,$3)`, id, state.MoveCount, column); err != nil { return game.State{}, err }
	if err = tx.Commit(ctx); err != nil { return game.State{}, err }
	return state, nil
}

func reconstruct(id string, columns []int) (game.State, error) {
	state, err := game.Replay(id, columns)
	// Bad stored history is an internal error, not an invalid browser request.
	if err != nil { return game.State{}, fmt.Errorf("invalid stored history: %v", err) }
	return state, nil
}
