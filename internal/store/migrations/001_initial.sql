CREATE TABLE games (
    id text PRIMARY KEY CHECK (id ~ '^[0-9a-f]{32}$'),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE moves (
    game_id text NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    move_number integer NOT NULL CHECK (move_number BETWEEN 1 AND 42),
    column_index integer NOT NULL CHECK (column_index BETWEEN 0 AND 6),
    PRIMARY KEY (game_id, move_number)
);
