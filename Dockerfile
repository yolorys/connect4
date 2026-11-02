FROM node:22-bookworm-slim AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm test && npm run build

FROM golang:1.26-bookworm AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/

FROM backend AS test
RUN go vet ./... && go test -race -count=1 ./...

FROM test AS build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/connect4 ./cmd/server

FROM scratch AS app
WORKDIR /app
COPY --from=build /out/connect4 ./connect4
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=frontend /src/web/dist ./web/dist
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=4s --start-period=75s --retries=3 CMD ["/app/connect4", "healthcheck"]
ENTRYPOINT ["/app/connect4"]
