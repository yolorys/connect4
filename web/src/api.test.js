import test from 'node:test';
import assert from 'node:assert/strict';
import { APIError, gamePath, request } from './api.js';

test('requests use no-store, JSON, and return the server state', async (t) => {
  t.mock.method(globalThis, 'fetch', async (url, options) => {
    assert.equal(url, '/api/games/example/moves');
    assert.equal(options.cache, 'no-store');
    assert.equal(options.headers['Content-Type'], 'application/json');
    assert.equal(options.method, 'POST');
    assert.deepEqual(JSON.parse(options.body), { column: 3, expectedMoveCount: 8 });
    return Response.json({ moveCount: 9 });
  });
  assert.deepEqual(await request('/api/games/example/moves', { method: 'POST', body: JSON.stringify({ column: 3, expectedMoveCount: 8 }) }), { moveCount: 9 });
});

test('conflicts preserve the backend error code for state recovery', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => Response.json({ code: 'stale_state', error: 'reload' }, { status: 409 }));
  await assert.rejects(request('/api/games/example'), (error) => error instanceof APIError && error.code === 'stale_state' && error.status === 409);
});

test('network failures never cause an automatic retry', async (t) => {
  const fetch = t.mock.method(globalThis, 'fetch', async () => { throw new TypeError('offline'); });
  await assert.rejects(request('/api/games/example/moves', { method: 'POST' }), (error) => error.code === 'network_error');
  assert.equal(fetch.mock.callCount(), 1);
});

test('unreadable responses remain recoverable failures', async (t) => {
  t.mock.method(globalThis, 'fetch', async () => new Response('<html>gateway error</html>', { status: 502 }));
  await assert.rejects(request('/api/games/example'), (error) => error.code === 'invalid_response');
});

test('game IDs are encoded as a path segment', () => {
  assert.equal(gamePath('abc/def?x=1'), '/api/games/abc%2Fdef%3Fx%3D1');
});
