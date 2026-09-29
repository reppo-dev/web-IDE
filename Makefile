.PHONY: all build build-backend build-frontend test run-backend run-frontend

all: test build

build: build-backend build-frontend

build-backend:
	cd backend && go build -o nexusd.exe ./cmd/nexusd

build-frontend:
	cd frontend && npm run build

test:
	cd backend && go test -v ./...

run-backend:
	cd backend && go run ./cmd/nexusd

run-frontend:
	cd frontend && npm run dev
