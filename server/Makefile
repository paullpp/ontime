.PHONY: run build tidy up down migrate-up migrate-down lint test

# Copy .env.example -> .env if missing
.env:
	cp .env.example .env

## run: run the server locally (requires postgres + redis running)
run: .env tidy
	go run ./cmd/server

## build: build the binary
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/server ./cmd/server

## tidy: tidy and download go modules
tidy:
	go mod tidy

## up: start all services via docker compose
up:
	docker compose up --build

## down: stop and remove containers
down:
	docker compose down

## migrate-up: run migrations against local postgres
migrate-up:
	migrate -path ./migrations -database "$$DATABASE_URL" up

## migrate-down: roll back last migration
migrate-down:
	migrate -path ./migrations -database "$$DATABASE_URL" down 1

## test: run all tests
test:
	go test ./... -race -count=1

## lint: run golangci-lint
lint:
	golangci-lint run ./...
