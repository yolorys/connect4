export const GAME_KEY = 'connect4.gameId';

export class APIError extends Error {
  constructor(message, code, status) {
    super(message);
    this.name = 'APIError';
    this.code = code;
    this.status = status;
  }
}

// A failed POST may have committed. Callers must retrieve the game before
// allowing another move, never automatically resend an uncertain move.
export async function request(path, options = {}) {
  let response;
  try {
    response = await fetch(path, {
      ...options,
      cache: 'no-store',
      headers: { 'Content-Type': 'application/json', ...options.headers },
      signal: options.signal ?? AbortSignal.timeout(12000),
    });
  } catch {
    throw new APIError('Could not reach the game server.', 'network_error', 0);
  }
  let body;
  try {
    body = await response.json();
  } catch {
    throw new APIError('The server returned an unreadable response.', 'invalid_response', response.status);
  }
  if (!response.ok) {
    throw new APIError(body.error || 'The request failed.', body.code, response.status);
  }
  return body;
}

export function gamePath(id) {
  return `/api/games/${encodeURIComponent(id)}`;
}
