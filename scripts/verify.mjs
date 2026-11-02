import assert from 'node:assert/strict';

const base = (process.env.BASE_URL || 'http://localhost:8080').replace(/\/$/, '');
async function call(path, options = {}, status = 200) {
  const response = await fetch(`${base}${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json' },
    signal: AbortSignal.timeout(10000),
  });
  assert.equal(response.status, status, `${options.method || 'GET'} ${path}`);
  return response.json();
}
try {
  await call('/readyz');
  const page = await fetch(base, { signal: AbortSignal.timeout(10000) });
  assert.equal(page.status, 200, 'frontend serves');
  assert.match(await page.text(), /Connect Four/);
  let game = await call('/api/games', { method: 'POST' }, 201);
  const path = `/api/games/${game.id}`;
  for (const column of [0,1,0,1,0,1,0]) {
    game = await call(`${path}/moves`, { method: 'POST', body: JSON.stringify({ column, expectedMoveCount: game.moveCount }) });
  }
  assert.equal(game.status, 'won');
  assert.equal(game.winner, 1);
  assert.deepEqual(await call(path), game, 'saved board matches after retrieval');
  const ended = await call(`${path}/moves`, { method: 'POST', body: JSON.stringify({ column: 2, expectedMoveCount: 7 }) }, 409);
  assert.equal(ended.code, 'game_finished');
  console.log(`Startup verified at ${base}: frontend, database, legal play, win, saved state, and rejection after game end.`);
} catch (error) {
  console.error(`Startup verification failed at ${base}: ${error.message}`);
  process.exitCode = 1;
}
