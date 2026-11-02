.PHONY: up down dev-db backend frontend test test-integration test-container e2e verify k8s-up k8s-down k8s-reset fmt

up:
	docker compose up --build --wait --wait-timeout 120

down:
	docker compose down

dev-db:
	docker compose -f compose.yaml -f compose.dev.yaml up -d --wait db

backend:
	go mod download
	DATABASE_URL="$${DATABASE_URL:-postgres://connect4:connect4_local@127.0.0.1:5432/connect4?sslmode=disable}" go run ./cmd/server

frontend:
	npm --prefix web ci
	npm --prefix web run dev

test:
	go vet ./...
	go test -race -count=1 ./...
	npm --prefix web test

test-integration:
	@test -n "$${TEST_DATABASE_URL}" || (echo 'Set TEST_DATABASE_URL to a dedicated test database.' >&2; exit 1)
	go test -race -count=1 -v ./internal/store

test-container:
	bash scripts/test-container.sh

e2e:
	npm --prefix web run test:e2e

verify:
	node scripts/verify.mjs

k8s-up:
	bash scripts/k8s-up.sh

k8s-down:
	kubectl --context kind-connect4 -n connect4 scale deployment/connect4 statefulset/postgres --replicas=0

k8s-reset:
	kind delete cluster --name connect4

fmt:
	gofmt -w cmd internal
