#!/bin/bash

set -e

echo " Деплой системи виявлення мін на DigitalOcean..."

# Перевірка наявності Docker та Docker Compose
if ! command -v docker &> /dev/null; then
    echo "Docker не встановлено. Встановіть Docker спочатку."
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo " Docker Compose не встановлено. Встановіть Docker Compose спочатку."
    exit 1
fi

# Створення директорій
echo " Створення необхідних директорій..."
mkdir -p init-db
mkdir -p ssl

# Зупинка існуючих контейнерів
echo "Зупинка існуючих контейнерів..."
docker-compose down --remove-orphans || true

# Очищення старих образів (опціонально)
read -p "🗑Видалити старі образи? (y/N): " clean_images
if [[ $clean_images =~ ^[Yy]$ ]]; then
    echo "Очищення старих образів..."
    docker image prune -f
    docker system prune -f
fi

# Збірка та запуск
echo "Збірка та запуск контейнерів..."
docker-compose up -d --build

# Очікування запуску бази даних
echo "Очікування запуску бази даних..."
sleep 10

# Перевірка статусу
echo "Перевірка статусу сервісів..."
docker-compose ps

echo "Перевірка API..."
sleep 5

for i in {1..10}; do
    if curl -f http://localhost:8080/api/v1/devices > /dev/null 2>&1; then
        echo "API готове до роботи!"
        break
    else
        echo "Очікування API... (спроба $i/10)"
        sleep 3
    fi
done

echo "Статус контейнерів:"
docker-compose ps

echo "Логи API (останні 20 рядків):"
docker-compose logs --tail=20 api-gateway

echo ""
echo "Деплой завершено!"
echo "API доступне за адресою: http://$(hostname -I | awk '{print $1}'):8080"
echo "Документація API: http://$(hostname -I | awk '{print $1}'):8080/api/v1/devices"
echo ""
echo "Корисні команди:"
echo "  Переглянути логи:     docker-compose logs -f"
echo "  Зупинити сервіси:     docker-compose down"
echo "  Перезапустити:        docker-compose restart"
echo "  Видалити все:         docker-compose down -v"
