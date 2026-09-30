#!/bin/bash
set -e

# Этот скрипт выполняется от имени суперпользователя $POSTGRES_USER
# и создает роль с паролем из переменной окружения APP_PASSWORD
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE ROLE app_user WITH LOGIN PASSWORD '$APP_PASSWORD';
EOSQL