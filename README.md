# Connect 4

A two-player classic Connect 4 game for one device, built with React, Go, and PostgreSQL.

## Run locally

Prerequisite: Docker with Compose installed and running. From the project directory:

```bash
docker compose up --build
```

Open **http://localhost:8080**. The build includes the frontend and backend; the
server applies database migrations automatically. Go, Node.js, and PostgreSQL
do not need to be installed on the host.

Stop with `docker compose down`; saved games stay in the database volume. The
browser remembers the current game ID, so refreshing restores it. Clearing
browser storage loses that reference.

Make is required for the helper commands below. If Docker requires `sudo`, use it
for Docker commands and Make targets that invoke Docker. The supplied database
credentials are for local development.

## Tests

Run the backend and PostgreSQL tests with race detection in containers:

```bash
make test-container
```

This uses a separate temporary database and removes it afterward.

For frontend and browser checks, install Node.js 20.19+ (20.x) or 22.12+, and npm.
Keep the app running at port 8080 and use another terminal in the project directory:

```bash
npm --prefix web ci
npm --prefix web test
(cd web && npx playwright install --with-deps chromium)
make e2e
make verify
```

`make e2e` runs seven browser tests covering play, refresh, new games, outcomes,
stale tabs, lost responses, and keyboard play on a small screen. Chromium
installation is needed for browser tests. `make verify` is a smoke check of the
running app and API; it creates one test game. Set `BASE_URL` for a different URL.

The app image build also runs `go vet`, Go unit tests with race detection,
JavaScript API tests, and the frontend production build.

## Kubernetes

Prerequisites: Docker, Make, [kubectl](https://kubernetes.io/docs/tasks/tools/),
and [kind](https://kind.sigs.k8s.io/docs/user/quick-start/).

```bash
docker compose down
make k8s-up
```

This creates or reuses the local cluster, builds and loads the app, waits for
readiness, and forwards **http://localhost:8080**. Leave that terminal open.
Ctrl+C stops forwarding; the workloads continue running.

There is one app replica and one PostgreSQL replica with persistent storage.
Compose and Kubernetes use separate databases. `make k8s-down` stops the
Kubernetes workloads while keeping their data. Keep using the same user for
Kubernetes commands, including `sudo` if you started the cluster with it.

## Verified

Confirmed on the host: Docker startup and browser play, backend/PostgreSQL tests
with race detection, all seven browser tests, and Kubernetes startup with game
persistence across replacement of the app and database pods.

Frontend builds and JavaScript API tests passed after the recent UI edits.
The updated browser suite was also rerun on the host and passed.
The standalone `make verify` smoke check also passed on the host.

## Decisions/tradeoffs

I kept the scope focused on a complete local game within the time expectation. It currently supports two players on one device. Online multiplayer is outside this version's scope.

Go serves both the API and React files. This keeps the application in one container and makes startup easier. The frontend and backend are deployed together so changing either requires rebuilding the app image.

Moves are stored and the board is rebuilt. The move history is the source of truth so there is no separate saved board or result to keep synchronized. This adds reconstruction work when loading a game but the history is limited to just 42 moves.

Stale moves are rejected. Transactions, a game-row lock, and an expected move count prevent conflicting requests from recording an unintended turn. The tradeoff here is that an outdated client must refresh its state before continuing.

With more time, I'd have a few more people try the game without instructions and then use their feedback to improve anything unclear. I'd also add a recentGames list so players can return to an earlier game after starting a new one. The games are already saved in PostgreSQL but the browser currently only remembers the active game.

## Time spent
Approximately 3.5 hours:
- Planning and design: ~1 hour
- Environment setup and troubleshooting: ~0.5 hour
- Implementation: ~1 hour
- Testing and documentation: ~1 hour

## AI usage
I used AI to help with planning, implementation, debugging, and documentation.
I reviewed the generated results and tested the application.
[View agent transcript](./transcripts/)