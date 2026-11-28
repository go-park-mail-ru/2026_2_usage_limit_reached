# OnlineConf — архитектура и план (backend)

Единый источник правды по хакатонскому заданию «OnlineConf» для бэкенда.

Референс — [onlineconf/onlineconf](https://github.com/onlineconf/onlineconf) (Mail.Ru):
берём идеи (история изменений, права на поддеревья, «продукт не зависит от
конфиг-сервиса на каждом запросе»), не код. Флагов и процентной раскатки у
референса нет — наш кейс ближе к LaunchDarkly/Unleash.

## 1. Границы

Конфигурируем **поведение продукта** (лимиты, включённость фич, кто их видит),
а не инфраструктуру: DSN, секреты и порты остаются в yaml/env, потому что нужны
для старта процесса. Секреты в OnlineConf не храним.

## 2. Карта системы

```
                        ┌──────────────────────────────────────┐
 админ-SPA ──HTTP─────► │  onlineconf-сервис (cmd/onlineconf)  │ ──► PostgreSQL
 (nginx фронтов)        │  HTTP: админ-API + /csrf             │     (oc_*)
 content, monetization  │  gRPC :50052 GetSnapshot             │
 ─gRPC-поллинг 15с────► └──────────────────────────────────────┘
```

Три типа клиентов:

| Клиент | Транспорт | Что получает |
|---|---|---|
| админ-SPA | HTTP `/onlineconf/admin/*` | CRUD, поиск, история, раскатка, проверка юзера |
| `content`, `monetization` | gRPC `GetSnapshot` | **сырые** данные скоупа; тогглы считают локально |
| продуктовый фронт | продуктовые `*/settings` | значения, которые сервисы отдают из своего кэша |

`/front` в onlineconf **не нужен**: у фронта нет такой ручки, флаги и настройки
он получает через `GET /donations/settings`, `GET /feed/settings` и т.п.
Скоуп `front` остаётся как обычные данные для сервисов.

## 3. Решения (ADR-конспект)

| # | Решение | Почему |
|---|---|---|
| 1 | Кэш в памяти сервисов, не файлы | потребитель — наш же Go-код; файл лишнее звено. Устойчивость та же: сервис лёг → живём на последнем снапшоте |
| 2 | **Short polling** 15–30с | бюджет «≤1 мин» закрывается арифметикой; гарантия = interval + timeout |
| 3 | Одна функция вычисления (`internal/clients/onlineconf/eval.go`) + golden-тест | сервис и потребители обязаны считать одинаково |
| 4 | Бакеты: `fnv64a(user_id + ":" + flag_key) % 10000 < pct*100` | детерминизм и монотонность: рост % никого не выключает |
| 5 | Порядок: аноним → force_off → force_on → rollout | исключение сильнее включения; PK `oc_flag_user` не даёт быть в обоих списках |
| 6 | Ревизия = `max(oc_history.id)`, мутации сериализуются `pg_advisory_xact_lock` | иначе поздно закоммитившаяся мутация с меньшим id прячется за max(id) |
| 7 | Срез читается в одной транзакции REPEATABLE READ, **ревизия до строк** | обратный порядок даёт пару «ревизия N, данные N−1» и теряет изменение |
| 8 | Параноидальное декодирование у потребителя (`contracts.JSON/Int64/String/Bool`) | битый конфиг не должен ронять запросы |
| 9 | Одна `oc_config` (JSONB + `value_type`) | конфиги создаются в рантайме; таблица на ключ = DDL на каждый конфиг |
| 10 | `oc_scope`: `id` PK + `UNIQUE(name)`, FK детей на `name` | имя — контрактный идентификатор, `id` нужен только истории |
| 11 | Админ-авторизация: живой SELECT роли и статуса из БД | отзыв прав мгновенный; трафик админки мизерный |
| 12 | Статику админ-SPA раздаёт **nginx фронтов** | у продукта так же; бэкенд — только API |
| 13 | Скоупы: `front`, `monetization`, `content` | совпадают с именами сервисов |
| 14 | Лимит доната: `payment.min_amount_minor`, копейки, скоуп `monetization`, дефолт 100 | имя/единицы совпадают с существующим валидатором; фронт получает через `/donations/settings` |

Точка рассинхрона (осознанно): окно ≤30–60с между изменением в админке и
применением у потребителей. Логика одна, данные одни — расходится только
свежесть. Это ровно бюджет задания.

## 4. Модель данных

[database/migrations/000016_create_onlineconf.up.sql](../database/migrations/000016_create_onlineconf.up.sql):
`oc_scope`, `oc_config`, `oc_flag`, `oc_flag_user`, `oc_history`.

- UUID-ключи, именованные констрейнты `uk_/chk_/fk_`, триггеры `set_updated_at`;
- `oc_history` — append-only (`reject_update`/`reject_delete`), `id BIGSERIAL`
  он же глобальная ревизия;
- сиды: скоупы, `('monetization','payment.min_amount_minor','100','scalar')`,
  `('content','feed_attachment_filter')`, плюс create-записи в `oc_history`
  (иначе после чистого деплоя ревизия 0 == `known_revision` 0 и клиент
  получит `not_modified`, не увидев сидов).

## 5. API

### 5.1 gRPC (для сервисов)

`OnlineConfService.GetSnapshot(scopes, known_revision) → {revision, not_modified, configs[], flags[]}`.
Конфиг `{scope, key, value}` — `value` это JSON-текст как в JSONB.
Флаг `{scope, key, rollout_percent, force_on_user_ids[], force_off_user_ids[]}`.
`known_revision = 0` → всегда полный срез.

### 5.2 Админ-API (HTTP, без префикса `/api` — его срезает nginx)

Авторизация: cookie-сессия → `AuthMiddleware` (идентификация из токена) →
`PermissionMiddleware` (живой SELECT роли и статуса из БД) → 401/403 →
`oc_history` в той же транзакции, что и мутация.

```
GET    /onlineconf/admin/access                       (PTN-71; no-store)
GET    /onlineconf/admin/scopes
POST   /onlineconf/admin/scopes                       {name, description}
GET    /onlineconf/admin/configs?scope=&q=
POST   /onlineconf/admin/configs                      {scope, key, value, value_type, description}
GET    /onlineconf/admin/configs/{scope}/{key}
PUT    /onlineconf/admin/configs/{scope}/{key}
DELETE /onlineconf/admin/configs/{scope}/{key}
GET    /onlineconf/admin/flags?scope=&q=
POST   /onlineconf/admin/flags                        {scope, key, description, rollout_percent?}
GET    /onlineconf/admin/flags/{scope}/{key}
PUT    /onlineconf/admin/flags/{scope}/{key}/rollout  {percent}
POST   /onlineconf/admin/flags/{scope}/{key}/force    {user_id, force: "on"|"off"}
DELETE /onlineconf/admin/flags/{scope}/{key}/force/{user_id}
DELETE /onlineconf/admin/flags/{scope}/{key}
POST   /onlineconf/admin/flags/check                  {scope, key, user_id}
GET    /onlineconf/admin/history?entity_type=&scope=&key=&limit=&cursor=
GET    /csrf
```

`value` — строка с JSON: `5000` для scalar, сериализованный JSON для json/array.
История — курсорная пагинация по конвенции `pkg/cursor` (kind `onlineconf_history`).

## 6. Интеграции

- **content**: `feed_attachment_filter` гейтит фильтр ленты — при выключенном
  флаге `filters.AttachmentType = ""`. Аноним → `uuid.Nil` → выключено.
- **monetization**: `payment.min_amount_minor` (скоуп `monetization`) —
  минимум доната; ниже лимита `400 {"error":"donation_amount_below_minimum"}`
  через `response.ErrorWithCode`; `GET /donations/settings` отдаёт
  `{"min_amount_minor": N}`.
- **identity**: роль читается из БД и кладётся в токен; админ создаётся
  бутстрапом из env (`ADMIN_*`), без миграции-сида.

## 7. Декомпозиция

- **Трек A** (сервис): миграция, repository/usecase/delivery(http+grpc), cmd,
  compose, Dockerfile, configs — сделано.
- **Трек B** (платформа): клиент, контракты, eval, интеграции content и
  monetization, фикс роли в identity — сделано коллегой.

## 8. Что дальше (бонусы по приоритету организаторов)

1. История изменений — **сделано** (таблица + ручка).
2. Проверка юзера на флаге — **сделано** (`POST .../flags/check` + `reason`).
3. Плавная раскатка (ступени, запрет прыжка на 100%) — не сделано.
4. Динамическое обновление фронта — закрыто через `*/settings` + поллинг.
5. Управление админами — не сделано (есть бутстрап и ручная правка роли).
6. LISTEN/NOTIFY ускорение, полный RBAC, группы — по остатку времени.

## 9. Явно не делаем

- файлы/cdb/updater-агент, symlink/case, шифрование секретов;
- WebSocket/SSE — поллинг закрывает бюджет;
- Redis — кило-снапшот в ОЗУ дешевле и не создаёт точку отказа;
- раздача статики админки бэкендом;
- `/front` — фронт ходит в продуктовые `*/settings`.
