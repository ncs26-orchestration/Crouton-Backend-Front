SHELL := /bin/bash

.PHONY: help up down logs ps build rebuild psql redis-cli migrate-up migrate-down migrate-status migrate-new seed fmt lint test clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

up: ## Start full stack in dev mode (hot reload)
	docker compose up --build

down: ## Stop stack, keep volumes
	docker compose down

logs: ## Tail logs for all services
	docker compose logs -f --tail=200

ps: ## List running services
	docker compose ps

psql: ## Open psql shell in the postgres container
	docker compose exec postgres psql -U app -d app

redis-cli: ## Open redis-cli in the redis container
	docker compose exec redis redis-cli

migrate-up: ## Apply all pending migrations (dbmate)
	@set -a && . ./.env > /dev/null 2>&1 && set +a && docker compose exec -e DATABASE_URL="$$DATABASE_URL" api dbmate --migrations-dir /app/migrations up

migrate-down: ## Roll back the last migration (dbmate)
	@set -a && . ./.env > /dev/null 2>&1 && set +a && docker compose exec -e DATABASE_URL="$$DATABASE_URL" api dbmate --migrations-dir /app/migrations down

migrate-status: ## Show which migrations have been applied (dbmate)
	@set -a && . ./.env > /dev/null 2>&1 && set +a && docker compose exec -e DATABASE_URL="$$DATABASE_URL" api dbmate --migrations-dir /app/migrations status

migrate-new: ## Create a new migration: make migrate-new name=create_orders
	@test -n "$(name)" || (echo "usage: make migrate-new name=<snake_case_name>" && exit 1)
	@set -a && . ./.env > /dev/null 2>&1 && set +a && docker compose exec -e DATABASE_URL="$$DATABASE_URL" api dbmate --migrations-dir /app/migrations new $(name)

seed: ## Seed the database (placeholder)
	@echo "TODO: wire a seed script into apps/api"

fmt: ## Format everything
	docker compose exec api gofmt -w .
	docker compose exec agent uv run ruff format .
	pnpm -r exec prettier --write .

lint: ## Lint all apps
	pnpm -r lint

test: ## Run tests across apps
	pnpm -r test

clean: ## Remove containers, volumes, images
	docker compose down -v --remove-orphans
	rm -rf node_modules .turbo apps/*/dist apps/*/build apps/api/bin apps/api/tmp apps/agent/.venv

# ── Infrastructure ─────────────────────────────────────────────────────────

tf-init: ## Init Terraform (run once)
	cd infra/terraform && terraform init

tf-plan: ## Preview infra changes (requires TF_VAR_project_id)
	cd infra/terraform && terraform plan

tf-apply: ## Provision VM on GCP (requires TF_VAR_project_id)
	cd infra/terraform && terraform apply

tf-destroy: ## Tear down GCP resources
	cd infra/terraform && terraform destroy

tf-ip: ## Print the VM external IP
	@cd infra/terraform && terraform output -raw vm_ip

ansible-deploy: ## Deploy the stack to the VM (fill inventory.ini first)
	ansible-playbook -i infra/ansible/inventory.ini infra/ansible/playbook.yml

# ── Local LLM (Ollama) ──────────────────────────────────────────────────────

ollama-start: ## Start Ollama server in the background
	ollama serve &

ollama-run: ## Run qwen2.5:7b interactively
	ollama run qwen2.5:7b

ollama-status: ## Check if Ollama is responding
	@curl -s http://localhost:11434/api/tags | python3 -m json.tool || echo "Ollama not running — try: make ollama-start"
