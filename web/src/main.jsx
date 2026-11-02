import React, { useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { GAME_KEY, gamePath, request } from './api.js';
import './style.css';

const names = { 1: 'Red', 2: 'Yellow' };
const emptyBoard = Array.from({ length: 6 }, () => Array(7).fill(0));

function App() {
  const [game, setGame] = useState(null);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [needsReload, setNeedsReload] = useState(false);
  const [savedID, setSavedID] = useState(null);
  const inFlight = useRef(true);

  useEffect(() => {
    let active = true;
    async function restore() {
      let id;
      try { id = localStorage.getItem(GAME_KEY); }
      catch { if (active) setNotice('Your browser cannot remember this game after a refresh.'); }
      if (active) setSavedID(id || null);
      if (id) {
        try {
          const state = await request(gamePath(id));
          if (active) setGame(state);
        } catch (failure) {
          if (active) {
            setError(failure.status === 404 ? 'Your saved game could not be found. Start a new game.' : 'Could not load your saved game. Reload to try again.');
            setNeedsReload(failure.status !== 404);
          }
        }
      }
      if (active) { inFlight.current = false; setBusy(false); }
    }
    restore();
    return () => { active = false; };
  }, []);

  async function startGame() {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setError('');
    try {
      const state = await request('/api/games', { method: 'POST' });
      setGame(state);
      setSavedID(state.id);
      setNeedsReload(false);
      try { localStorage.setItem(GAME_KEY, state.id); setNotice(''); }
      catch { setNotice('Your browser cannot remember this game after a refresh.'); }
    } catch {
      setError('Could not start a new game. Please try again.');
    } finally { inFlight.current = false; setBusy(false); }
  }

  async function reload() {
    const id = game?.id || savedID;
    if (!id || inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setError('');
    try {
      setGame(await request(gamePath(id)));
      setNeedsReload(false);
    } catch (failure) {
      setNeedsReload(failure.status !== 404);
      if (failure.status === 404) setGame(null);
      setError(failure.status === 404 ? 'This game could not be found. Start a new game.' : 'Could not reload the game. Please try again.');
    } finally { inFlight.current = false; setBusy(false); }
  }

  async function drop(column) {
    if (inFlight.current || !game || needsReload || game.status !== 'playing' || !game.availableColumns.includes(column)) return;
    inFlight.current = true;
    setBusy(true);
    setError('');
    try {
      const state = await request(`${gamePath(game.id)}/moves`, {
        method: 'POST',
        body: JSON.stringify({ column, expectedMoveCount: game.moveCount }),
      });
      setGame(state);
    } catch (failure) {
      if (['stale_state', 'column_full', 'game_finished'].includes(failure.code)) {
        try {
          setGame(await request(gamePath(game.id)));
          setError('The board changed. The current game is shown; choose again if it is still in progress.');
        } catch {
          setNeedsReload(true);
          setError('Could not reload the current board. Reload the game before continuing.');
        }
      } else {
        setNeedsReload(true);
        setError('We could not confirm that move. Reload the game before continuing.');
      }
    } finally { inFlight.current = false; setBusy(false); }
  }

  const playing = game?.status === 'playing';
  const player = game?.winner || game?.nextPlayer || 0;
  const title = busy && !game ? 'Getting ready…' : needsReload ? 'Reconnect to continue' : !game ? 'Ready when you are.' : game.status === 'won' ? `${names[game.winner]} wins!` : game.status === 'draw' ? 'It’s a draw.' : `${names[game.nextPlayer]}’s turn`;
  const subtitle = !game ? 'Start a game. Red takes the first turn.' : game.status === 'won' ? 'Four connected. Nicely played.' : game.status === 'draw' ? 'A full board. A well-matched pair.' : busy ? 'Saving your move…' : needsReload ? 'Your game is saved on the server.' : 'Choose a column to drop your disc.';

  return (
    <main className="app">
      <section className="game-card" aria-label="Connect Four game">
        <div className="game-heading">
          <h1>Play Connect 4.</h1>
        </div>

        <div className="status-bar" role="status" aria-live="polite" aria-atomic="true">
          <span className={`turn-disc player-${player}`} aria-hidden="true" />
          <div><h2>{title}</h2><p>{subtitle}</p></div>
        </div>

        <div className="board-shell">
          <div className="board" role="group" aria-label="Game board: 6 rows and 7 columns" aria-busy={busy}>
            {Array.from({ length: 7 }, (_, column) => {
              const full = Boolean(game && game.board[0][column] !== 0);
              const disabled = busy || needsReload || !playing || !game.availableColumns.includes(column);
              return (
                <button key={column} type="button" className="column" disabled={disabled}
                  aria-label={full ? `Column ${column + 1} is full` : `Drop in column ${column + 1}`}
                  onClick={() => drop(column)}>
                  {(game?.board || emptyBoard).map((row, rowIndex) => {
                    const last = game?.lastMove?.row === rowIndex && game?.lastMove?.column === column;
                    return <span key={`${rowIndex}-${row[column]}-${last}`} className={`cell player-${row[column]}${last ? ' last-disc' : ''}`}
                      data-row={rowIndex} data-column={column} data-player={row[column]} data-last={last ? 'true' : undefined}
                      role="img" aria-label={`Row ${rowIndex + 1}, column ${column + 1}: ${row[column] ? names[row[column]] : 'empty'}${last ? ', last move' : ''}`} />;
                  })}
                </button>
              );
            })}
          </div>
          <div className="board-foot" aria-hidden="true" />
        </div>

        {error && <div className="error-message" role="alert"><p>{error}</p>{needsReload && <button className="text-button" onClick={reload} disabled={busy}>Reload game</button>}</div>}
        {notice && <p className="storage-notice" role="status">{notice}</p>}

        <div className="game-footer">
          <div className="players" aria-label="Players">
            <span><i className="legend-disc player-1" aria-hidden="true" /> Red</span>
            <span><i className="legend-disc player-2" aria-hidden="true" /> Yellow</span>
          </div>
          <button className="primary-button" onClick={startGame} disabled={busy}>{game || savedID ? 'New game' : 'Start game'}<span aria-hidden="true">↗</span></button>
        </div>
      </section>
    </main>
  );
}

createRoot(document.getElementById('root')).render(<React.StrictMode><App /></React.StrictMode>);
