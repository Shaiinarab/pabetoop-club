.PHONY: all deps tidy build run dev client test backup-check backup-check-test up down test-up test-down test-logs test-smoke clean health

PORT    ?= 8090
COMPOSE ?= docker compose

all: build

## deps: resolve Go modules (single binary)
deps:
	go mod tidy

## client: install npm deps (typescript@7 tsgo, date-fns-jalali, tailwindcss)
client:
	@cd client && npm install

## build: compile single portable binary
build: deps
	go build -trimpath -o pabetoop-club ./cmd/app

run: build   ## run dev server (stub mode), loading .env for SITE_* branding
	@ENV_FILE=./.env . ./tools/load-env.sh; STUB=1 exec ./pabetoop-club serve --http=127.0.0.1:$(PORT)

dev: client  ## watch TS
	@cd client && npx tsgo --watch

test:        ## unit tests + type-check
	go test ./...
	@cd client && npx tsgo --noEmit --strict

backup-check: ## build standalone Rust backup-integrity utility
	@cd tools/backup-check && cargo build --release

backup-check-test: ## test standalone Rust backup-integrity utility
	@cd tools/backup-check && cargo test

tidy: deps
up:
	$(COMPOSE) up -d --build
down:
	$(COMPOSE) down

test-up: ## start isolated synthetic Docker test deployment
	$(COMPOSE) -f docker-compose.test.yml up -d --build --wait

test-down: ## stop test deployment and remove synthetic test data
	$(COMPOSE) -f docker-compose.test.yml down -v --remove-orphans

test-logs: ## follow logs from the test deployment
	$(COMPOSE) -f docker-compose.test.yml logs -f --tail=100

test-smoke: ## run manager and guardian smoke workflow against the test deployment
	TEST_APP_PORT=$${TEST_APP_PORT:-8091} ./scripts/test-deploy-smoke.sh
clean:
	rm -rf pabetoop-club pb_data client/dist client/node_modules go.sum

health:
	curl -sf http://127.0.0.1:$(PORT)/_healthz || echo "down"
