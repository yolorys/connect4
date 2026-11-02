// Package game contains the rules of Connect 4, independent of HTTP and storage.
package game

import "errors"

const (
	Rows = 6
	Columns = 7
	Playing = "playing"
	Won = "won"
	Draw = "draw"
)

var (
	ErrColumn = errors.New("choose a column between 0 and 6")
	ErrFull = errors.New("that column is full")
	ErrFinished = errors.New("this game has ended")
	ErrStale = errors.New("the game changed; reload its current state")
	ErrNotFound = errors.New("game not found")
)

type Move struct {
	Row int `json:"row"`
	Column int `json:"column"`
	Player int `json:"player"`
}

// Row zero is the top of the board. Player 1 is red; player 2 is yellow.
// NextPlayer is zero after the game ends. Winner is zero unless someone won.
type State struct {
	ID string `json:"id"`
	Board [Rows][Columns]int `json:"board"`
	MoveCount int `json:"moveCount"`
	Status string `json:"status"`
	NextPlayer int `json:"nextPlayer"`
	Winner int `json:"winner"`
	LastMove *Move `json:"lastMove"`
	AvailableColumns []int `json:"availableColumns"`
}

func New(id string) State {
	s := State{ID: id, Status: Playing, NextPlayer: 1}
	s.updateAvailable()
	return s
}

// Replay reconstructs and validates the complete history, at most 42 moves.
func Replay(id string, columns []int) (State, error) {
	s := New(id)
	for _, column := range columns {
		if err := s.Drop(column); err != nil {
			return State{}, err
		}
	}
	return s, nil
}

// Drop changes the state only after a legal move has been established.
func (s *State) Drop(column int) error {
	if s.Status != Playing { return ErrFinished }
	if column < 0 || column >= Columns { return ErrColumn }
	row := Rows - 1
	for row >= 0 && s.Board[row][column] != 0 { row-- }
	if row < 0 { return ErrFull }

	player := s.NextPlayer
	s.Board[row][column] = player
	s.MoveCount++
	s.LastMove = &Move{Row: row, Column: column, Player: player}
	if s.connected(row, column, player) {
		s.Status, s.Winner, s.NextPlayer = Won, player, 0
	} else if s.MoveCount == Rows * Columns {
		s.Status, s.NextPlayer = Draw, 0
	} else {
		s.NextPlayer = 3 - player
	}
	s.updateAvailable()
	return nil
}

func (s *State) updateAvailable() {
	s.AvailableColumns = make([]int, 0, Columns)
	if s.Status == Playing {
		for column := 0; column < Columns; column++ {
			if s.Board[0][column] == 0 {
				s.AvailableColumns = append(s.AvailableColumns, column)
			}
		}
	}
}

func (s *State) connected(row, column, player int) bool {
	for _, direction := range [][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}} {
		count := 1
		for _, sign := range []int{-1, 1} {
			for step := 1; ; step++ {
				r := row + sign * step * direction[0]
				c := column + sign * step * direction[1]
				if r < 0 || r >= Rows || c < 0 || c >= Columns || s.Board[r][c] != player { break }
				count++
			}
		}
		if count >= 4 { return true }
	}
	return false
}
