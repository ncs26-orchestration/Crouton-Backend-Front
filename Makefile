.PHONY: help install dev sqlc migrate new-migration up down logs build-go test-go schema

# Local DATABASE_URL for host-side dbmate/sqlc against the dev compose postgres.
DATABASE_URL ?= postgres://app:app@127.0.0.1:55432/app?sslmode=disable
export DATABASE_URL

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

install: ## Install web dependencies
	pnpm install

dev: ## Run the web app dev server (expects a local API or compose stack)
	pnpm --filter web dev

up: ## Build and start the full stack locally (http://localhost:8088)
	docker compose up -d --build

down: ## Stop the local stack
	docker compose down

logs: ## Tail local stack logs
	docker compose logs -f

sqlc: ## Regenerate Go code from SQL
	sqlc generate

migrate: ## Apply migrations to the dev compose postgres
	dbmate up

new-migration: ## Create a migration: make new-migration name=add_widget
	dbmate new $(name)

schema: ## Dump the dev compose postgres schema to db/schema.sql
	docker compose exec -T postgres pg_dump -U app -d app --schema-only --no-owner --no-privileges > db/schema.sql

build-go: ## Build the Go backend
	cd apps/api && go build ./...

test-go: ## Test the Go backend
	cd apps/api && go test ./...
