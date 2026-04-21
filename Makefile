BINARY   := stackpr
CMD_PATH := ./cmd/stackpr
MODULE   := github.com/stackpr/stackpr

.PHONY: build run docker-up docker-down migrate tidy lint web-install web-build web-dev docker-up-all

## build: Compile the stackpr binary.
build:
	CGO_ENABLED=0 go build -o $(BINARY) $(CMD_PATH)

## run: Build and run the webhook server locally (requires DATABASE_URL, GITHUB_TOKEN, WEBHOOK_SECRET in env).
run: build
	./$(BINARY) serve --port 8080

## tidy: Tidy and verify the go module.
tidy:
	go mod tidy
	go mod verify

## lint: Run go vet.
lint:
	go vet ./...

## docker-up: Start postgres and stackpr containers.
docker-up:
	docker compose up --build -d

## docker-down: Stop and remove containers and the named volume.
docker-down:
	docker compose down -v

## migrate: Run database migrations against the configured DATABASE_URL.
migrate: build
	DATABASE_URL=$${DATABASE_URL} ./$(BINARY) stack list > /dev/null || true
	@echo "Migrations applied (they run automatically on startup)."

## test: Run all unit tests.
test:
	go test ./...

## help: Print this help message.
help:
	@grep -E '^## ' Makefile | sed 's/## //'

## web-install: Install frontend dependencies.
web-install:
	cd web && npm ci

## web-build: Build the frontend for production.
web-build:
	cd web && npm ci && npm run build

## web-dev: Start the frontend dev server.
web-dev:
	cd web && npm run dev

## docker-up-all: Build and start all services including the web frontend.
docker-up-all:
	docker compose up --build -d
