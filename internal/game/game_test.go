package game

import (
	"errors"
	"reflect"
	"testing"
)

func TestGravityTurnsAndLastMove(t *testing.T) {
	s := New("test")
	if s.NextPlayer != 1 || s.MoveCount != 0 || s.LastMove != nil || len(s.AvailableColumns) != 7 { t.Fatalf("bad initial state: %+v", s) }
	for turn, column := range []int{3, 3, 5} {
		if err := s.Drop(column); err != nil { t.Fatal(err) }
		wantRow := 5
		if turn == 1 { wantRow = 4 }
		wantPlayer := turn%2 + 1
		if *s.LastMove != (Move{Row: wantRow, Column: column, Player: wantPlayer}) { t.Fatalf("last move: %+v", s.LastMove) }
		if s.Board[wantRow][column] != wantPlayer || s.NextPlayer != 3-wantPlayer { t.Fatal("gravity or turn mismatch") }
	}
	replayed, err := Replay("test", []int{3, 3, 5})
	if err != nil || !reflect.DeepEqual(s, replayed) { t.Fatalf("replay mismatch: %+v %v", replayed, err) }
}

func TestInvalidMovesDoNotChangeState(t *testing.T) {
	s := New("test")
	for i := 0; i < Rows; i++ { if err := s.Drop(2); err != nil { t.Fatal(err) } }
	if len(s.AvailableColumns) != 6 { t.Fatalf("full column still available: %v", s.AvailableColumns) }
	for _, column := range []int{-1, 7, 100, 2} {
		before := s
		want := ErrColumn
		if column == 2 { want = ErrFull }
		if err := s.Drop(column); !errors.Is(err, want) { t.Fatalf("column %d: %v", column, err) }
		if !reflect.DeepEqual(s, before) { t.Fatal("rejected move changed state") }
	}
}

func TestWinsAndFinishedGames(t *testing.T) {
	tests := []struct { name string; columns []int; winner int }{
		{"horizontal", []int{0, 0, 1, 1, 2, 2, 3}, 1},
		{"vertical", []int{0, 1, 0, 1, 0, 1, 0}, 1},
		{"diagonal ascending", []int{0, 1, 1, 2, 4, 2, 2, 3, 4, 3, 5, 3, 3}, 1},
		{"diagonal descending", []int{6, 5, 5, 4, 2, 4, 4, 3, 2, 3, 1, 3, 3}, 1},
		{"yellow", []int{6, 0, 6, 1, 5, 2, 5, 3}, 2},
	}
	for _, tt := range tests { t.Run(tt.name, func(t *testing.T) {
		s, err := Replay("test", tt.columns)
		if err != nil { t.Fatal(err) }
		if s.Status != Won || s.Winner != tt.winner || s.NextPlayer != 0 || len(s.AvailableColumns) != 0 { t.Fatalf("bad win: %+v", s) }
		before := s
		if err := s.Drop(6); !errors.Is(err, ErrFinished) { t.Fatalf("move after win: %v", err) }
		if !reflect.DeepEqual(before, s) { t.Fatal("finished game changed") }
	}) }
}

func TestDrawAndLastSpaceWin(t *testing.T) {
	tests := []struct { name string; columns []int; status string; winner int }{
		{"draw", []int{6,6,6,5,2,2,5,1,4,3,3,6,3,4,0,4,1,0,4,4,1,6,5,1,0,5,1,4,3,1,5,3,0,6,5,2,2,0,2,3,2,0}, Draw, 0},
		{"win on final space", []int{2,3,2,2,3,4,2,1,1,2,0,2,5,0,1,6,5,6,5,1,6,3,1,3,6,1,3,3,0,6,0,0,0,5,6,4,4,5,5,4,4,4}, Won, 2},
	}
	for _, tt := range tests { t.Run(tt.name, func(t *testing.T) {
		s, err := Replay("test", tt.columns)
		if err != nil { t.Fatal(err) }
		if s.MoveCount != 42 || s.Status != tt.status || s.Winner != tt.winner || s.NextPlayer != 0 || len(s.AvailableColumns) != 0 { t.Fatalf("bad full board: %+v", s) }
		if err := s.Drop(0); !errors.Is(err, ErrFinished) { t.Fatal("accepted a move on a finished board") }
	}) }
}

func TestReplayRejectsImpossibleHistory(t *testing.T) {
	for _, columns := range [][]int{{7}, {0,0,0,0,0,0,0}, {0,1,0,1,0,1,0,2}} {
		if _, err := Replay("test", columns); err == nil { t.Fatalf("accepted history %v", columns) }
	}
}

func TestConnectionsRequireSamePlayerAndNoGaps(t *testing.T) {
	s := New("test")
	s.Board[5] = [Columns]int{1,1,2,1,0,1,1}
	if s.connected(5, 1, 1) { t.Fatal("connected through another player") }
	s.Board[5] = [Columns]int{1,1,0,1,1,0,0}
	if s.connected(5, 1, 1) { t.Fatal("connected through a gap") }
	s.Board[5] = [Columns]int{1,1,1,1,1,0,0}
	if !s.connected(5, 2, 1) { t.Fatal("missed a run longer than four") }
}
