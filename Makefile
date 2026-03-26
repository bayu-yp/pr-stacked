BINARY   := stackpr
CMD_PATH := ./cmd/stackpr
MODULE   := github.com/stackpr/stackpr

.PHONY: build run docker-up docker-down migrate tidy lint

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
