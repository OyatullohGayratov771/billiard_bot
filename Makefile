include .env
export

# ===================== LOCAL =====================

run:
	cd bot-gateway && go run ./cmd

build:
	cd bot-gateway && go build -o ../bin/billiard_bot ./cmd

tidy:
	cd bot-gateway && go mod tidy

# ===================== DOCKER =====================

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f bot

restart:
	docker compose restart bot

ps:
	docker compose ps

# ===================== DB =====================

db-shell:
	docker compose exec postgres psql -U $${DB_USER:-postgres} -d $${DB_NAME:-billiard_bot}

db-reset:
	docker compose exec postgres psql -U $${DB_USER:-postgres} -c "DROP DATABASE IF EXISTS $${DB_NAME:-billiard_bot}; CREATE DATABASE $${DB_NAME:-billiard_bot};"

# ===================== HELP =====================

help:
	@echo ""
	@echo "  make run       — Botni lokal ishga tushirish"
	@echo "  make build     — Binary yaratish"
	@echo "  make tidy      — go mod tidy"
	@echo "  make up        — Docker bilan ishga tushirish"
	@echo "  make down      — Docker to'xtatish"
	@echo "  make logs      — Bot loglarini ko'rish"
	@echo "  make db-shell  — PostgreSQL shell"
	@echo ""

.PHONY: run build tidy up down logs restart ps db-shell db-reset help
