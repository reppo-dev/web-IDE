.PHONY: all build-backend test-backend run-backend build-frontend

all: test-backend build-backend

build-backend:
	cd backend && go build -o nexusd.exe ./cmd/nexusd

test-backend:
	cd backend && go test -v ./...

run-backend:
	cd backend && go run ./cmd/nexusd
