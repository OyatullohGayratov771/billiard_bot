#!/bin/bash
# Avtomatik yangilash skripti
# crontab ga qo'shish: 0 3 * * * /opt/billiard_bot/scripts/update.sh >> /var/log/billiard_update.log 2>&1

set -e
cd "$(dirname "$0")/.."

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Yangilanish boshlanmoqda..."

# Git dan yangi versiyani olish
git fetch origin main
LOCAL=$(git rev-parse HEAD)
REMOTE=$(git rev-parse origin/main)

if [ "$LOCAL" = "$REMOTE" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Yangilik yo'q. Chiqish."
    exit 0
fi

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Yangi commit topildi. Pull qilinmoqda..."
git pull origin main

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Docker rebuild boshlanmoqda..."
docker-compose build --no-cache tournament-service bot

echo "[$(date '+%Y-%m-%d %H:%M:%S')] Servislar qayta ishga tushirilmoqda..."
docker-compose up -d --no-deps tournament-service bot

echo "[$(date '+%Y-%m-%d %H:%M:%S')] ✅ Yangilanish tugadi. Commit: $REMOTE"
