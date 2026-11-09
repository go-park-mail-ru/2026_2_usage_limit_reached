# Подключается отдельно от боевого окружения: include seed/Makefile.seed.mk
.PHONY: seed-avatars
seed-avatars: ## Загрузить аватарки seed-пользователей в S3 (идемпотентно)
	docker compose -f docker-compose.yaml -f seed/docker-compose.seed-avatars.yml run --rm seed-avatars
