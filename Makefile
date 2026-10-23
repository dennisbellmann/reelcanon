.DEFAULT_GOAL := start
CANON ?= imdb-top-250

.PHONY: start stop restart help install dev api admin migrate import-canon enrich test-e2e frontend-dev build test ci deploy fmt fmt-check lint clean mobile-check mobile-test android-debug android-device-test android-bundle

.PHONY: dev-mail accounts-local

start:
	@powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/start-local.ps1 -Rebuild

stop:
	@powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/stop-local.ps1

restart:
	@$(MAKE) stop
	@$(MAKE) start

help:
	@echo "Local Windows: make (or make start), make stop, make restart"
	@echo "Targets: install dev frontend-dev api admin dev-mail accounts-local migrate import-canon enrich build test test-e2e ci deploy fmt fmt-check lint clean"
	@echo "Mobile: mobile-check mobile-test android-debug android-device-test android-bundle (MOBILE_ARGS=...)"

install:
	npm --prefix frontend ci
	cd backend && go mod download

dev:
	$(MAKE) frontend-dev

api:
	cd backend && go run ./cmd/api

dev-mail:
	cd backend && go run ./cmd/mailbox

accounts-local:
	cd backend && go run ./cmd/setup-local-accounts

admin:
	cd backend && go run ./cmd/admin

migrate:
	cd backend && go run ./cmd/migrate -dir ../database/migrations

import-canon:
	cd backend && go run ./cmd/import-canon -file "../$(FILE)"

frontend-dev:
	cd frontend && npm run dev

build:
	node scripts/build-backend.mjs
	npm --prefix frontend run build

test:
	cd backend && go test ./...
	cd frontend && npm test

enrich:
	cd backend && go run ./cmd/enrich -canon $(CANON)

test-e2e:
	cd frontend && npm run test:e2e

ci: fmt-check lint test build

# Deploys the latest commit already on main through the GitHub Actions CI/CD workflow.
# GitHub CLI (gh) must be installed and authenticated for this repository.
deploy:
	gh workflow run ci.yml --ref main
	@echo "CI/CD for main queued. Follow it with: gh run watch"

fmt:
	node scripts/check-gofmt.mjs --write
	cd frontend && npm run format

fmt-check:
	node scripts/check-gofmt.mjs
	cd frontend && npm run format:check

lint:
	cd backend && go vet ./...
	cd frontend && npm run lint

clean:
	cd frontend && npm run clean

mobile-check:
	node scripts/run-mobile.mjs check $(MOBILE_ARGS)

mobile-test:
	node scripts/run-mobile.mjs test $(MOBILE_ARGS)

android-debug:
	node scripts/run-mobile.mjs debug $(MOBILE_ARGS)

android-device-test:
	node scripts/run-mobile.mjs device $(MOBILE_ARGS)

android-bundle:
	node scripts/run-mobile.mjs bundle $(MOBILE_ARGS)
