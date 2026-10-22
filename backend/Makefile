.PHONY: run-api run-admin migrate build lint

run-api:
	go run ./cmd/api

run-admin:
	go run ./cmd/admin

migrate:
	go run ./cmd/migrate -dir ../database/migrations

build:
	node ../scripts/build-backend.mjs

lint:
	go vet ./...
