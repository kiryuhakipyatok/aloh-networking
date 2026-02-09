-include .env
export
version=
name=

all: build test

build:
	@go build -o main.exe cmd/app/main.go

run:
	@go run cmd/app/main.go

docker-run-app:
	@docker compose -f docker-compose.yaml up -d --build

docker-run-app-456:
	@docker compose -f docker-compose.yaml up -d user-456 --build

docker-down:
	@docker compose down

docker-build:
	@docker compose build --no-cache

test:
	@go test ./... -v

build-lib:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o libnetworking.dll -buildmode=c-shared -ldflags "-s -w -extldflags '-Wl,--output-def,libnetworking.def'" cmd/api/main.go
	go build -o libnetworking.so -buildmode=c-shared cmd/api/main.go
	gendef libnetworking.dll