-include .env
export
version=
name=

all: build test

build:
	@go build -o main.exe cmd/app/main.go

run:
	@go run cmd/app/main.go

docker-run-app-123:
	@docker compose -f docker-compose.yaml up -d user-123 --build

docker-run-app-456:
	@docker compose -f docker-compose.yaml up -d user-456 --build

docker-down:
	@docker compose down

docker-build:
	@docker compose build --no-cache

test:
	@go test ./... -v