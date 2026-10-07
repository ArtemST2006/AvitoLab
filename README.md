# Trip Service

HTTP-сервис поездок курса «Разработка микросервисов на Go» (лабораторная работа 1).
Создаёт поездку, отдаёт её и завершает. Данные хранятся в PostgreSQL, HTTP-слой
сгенерирован из OpenAPI-контракта.

Стек: `net/http` + `chi`, `oapi-codegen`, `pgx/v5` (`pgxpool`), `squirrel`, `goose`, `cleanenv`, `slog`.

## Запуск

Нужны Go 1.26+ и [`tripgoctl`](https://github.com/course-go-autumn-2026/course-infra).

```bash
# 1. Поднять окружение (PostgreSQL). tripgoctl создаёт .env с DATABASE_URL
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect

# 2. Скачать зависимости и накатить миграции
make install
make migrate

#2.5 
Добавтье в .env: **CONFIG_PATH=config/local.yaml** - путь до локального конфига

# 3. Запустить сервис
make run
```


### Команды Makefile

| Команда | Что делает |
|---|---|
| `make help` | список команд |
| `make install` | скачать зависимости |
| `make build` | собрать бинарь в `bin/trip-service` |
| `make run` | запустить сервис (параметры — через `ARGS`) |
| `make generate` | сгенерировать `internal/generated/api.gen.go` из `contracts/openapi/trip-service.openapi.yaml` |
| `make migrate` / `migrate-down` / `migrate-status` | применить / откатить последнюю / показать статус миграций |
| `make migrate-create NAME=...` | создать новую SQL-миграцию |
| `make format` / `make lint` | форматирование и линтеры |
| `make test` | тесты с race detector и отчёт покрытия |

## Конфигурация

Все параметры читаются из окружения (Makefile подхватывает `.env`). Если задан
`CONFIG_PATH`, сначала читается YAML-файл, затем env-переменные его перекрывают.
Обязательные параметры проверяются на старте: без них сервис не запустится.
Если PostgreSQL недоступна (`Ping` на старте не прошёл), сервис тоже не стартует.

| Переменная | Обязательная | По умолчанию | Описание |
|---|---|---|---|
| `HTTP_ADDR` | да | — | адрес HTTP-сервера, например `:8080` |
| `LOG_LEVEL` | да | — | `debug` / `info` / `warn` / `error` |
| `SHUTDOWN_TIMEOUT` | да | — | сколько ждать завершения активных запросов при остановке |
| `ENV` | нет | `local` | `local` — цветной текстовый лог, `dev` / `prod` — JSON |
| `CONFIG_PATH` | нет | — | путь к YAML-конфигу, например `config/local.yaml` |
| `HTTP_READ_HEADER_TIMEOUT` | нет | `5s` | `http.Server.ReadHeaderTimeout` |
| `HTTP_READ_TIMEOUT` | нет | `10s` | `http.Server.ReadTimeout` |
| `HTTP_WRITE_TIMEOUT` | нет | `10s` | `http.Server.WriteTimeout` |
| `HTTP_IDLE_TIMEOUT` | нет | `60s` | `http.Server.IdleTimeout` |
| `DATABASE_URL` | да | — | строка подключения к PostgreSQL |
| `DATABASE_MAX_CONNS` | да | — | максимальный размер пула |
| `DATABASE_MIN_CONNS` | да | — | минимальный размер пула |
| `DATABASE_MAX_CONN_LIFETIME` | да | — | время жизни соединения |
| `DATABASE_CONNECT_TIMEOUT` | да | — | таймаут подключения |
| `DATABASE_QUERY_TIMEOUT` | да | — | таймаут одного запроса к БД |

Пример значений — в [`.env.example`](.env.example).

## API

Контракт: [`contracts/openapi/trip-service.openapi.yaml`](contracts/openapi/trip-service.openapi.yaml).
Ошибки отдаются в формате `application/problem+json`.

| Метод и путь | Ответы |
|---|---|
| `POST /api/v1/trips` | `201` поездка создана (+ `Location`), `200` повтор по `Idempotency-Key`, `400`, `409 driver_busy`, `409 idempotency_conflict`, `500` |
| `GET /api/v1/trips/{tripId}` | `200`, `400`, `404 trip_not_found`, `500` |
| `POST /api/v1/trips/{tripId}/finish` | `200`, `400`, `404 trip_not_found`, `409 trip_completed`, `500` |
| `GET /health` | `200`, пока процесс жив; в БД не ходит |
| `GET /ready` | `200`, если БД доступна, иначе `503` |


## Структура

```
.
├── api
├── cmd
│   └── trip-service        # точка входа: конфиг, логгер, пул, роутер, graceful shutdown
├── config                  # YAML-конфиг для локального запуска
├── contracts               # OpenAPI-контракт и схема данных
│   ├── asyncapi
│   ├── events
│   ├── openapi
│   └── proto
│       ├── push
│       │   └── v1
│       └── trip
│           └── v1
├── internal
│   ├── config              # загрузка конфигурации
│   ├── generated
│   ├── http-server
│   │   ├── apierr          # обработчики и problem+json ошибки
│   │   └── handlers
│   ├── lib
│   │   └── logger
│   │       ├── handlers
│   │       └── sl
│   ├── schemas             # кастомные типы
│   └── storage             # интерфейсы хранилища
│       └── postgres        # пул, менеджер транзакций, репозитории
├── migrations
```

## Решения

### Уровень изоляции — Read Committed

Транзакции нутри менеджера по умолчанию открываются с read commited. Инварианты сервиса на ограничениях схемы:

- запрет двух активных поездок — частичный уникальный индекс;
- защита от двойного завершения — `SELECT ... FOR UPDATE`.

Поэтому более строгие уровни (`REPEATABLE READ`, `SERIALIZABLE`) ничего бы не
добавили, но потребовали бы повторять транзакции при ошибках.

### Менеджер транзакций

`postgres.TransactionManager` (`internal/storage/postgres/tx.go`) реализует
интерфейс `TxManager` с единственным методом
`Do(ctx, fn func(ctx) error) error`:

- `Do` открывает транзакцию через `pool.BeginTx`, кладёт `pgx.Tx` в `context`
  под приватным ключом и вызывает `fn` с этим контекстом;
- `fn` вернула `nil` — `COMMIT`; вернула ошибку — `ROLLBACK` и ошибка
  пробрасывается наружу; паника — `ROLLBACK` и паника пробрасывается дальше;
- если в контексте уже есть транзакция (вложенный `Do`), `fn` выполняется в ней,
  вторая транзакция не открывается;
- репозитории получают исполнителя через `giver(ctx, pool)`: транзакция из
  контекста, если она есть, иначе пул. Транзакция не передаётся аргументом
  метода, поэтому обработчики не знают про `pgx`. Вызов репозитория вне `Do`
  просто выполняется на пуле в автокоммите.

Создание поездки выполняется в одном `Do`: `INSERT` в `trips` и `INSERT` в
`trip_status_history`. Если вторая вставка падает, откатывается и первая.
Завершение поездки тоже идёт в одном `Do`: блокировка строки, `UPDATE` и запись
в историю.

### Не больше одной активной поездки на водителя

Правило закреплено в схеме частичным уникальным индексом:

```sql
CREATE UNIQUE INDEX trips_driver_active_uidx ON trips (driver_id) WHERE status = 'active';
```

При конкурентных `create` на одного водителя PostgreSQL пропускает только одну
вставку. Остальные получают ошибку `23505`.

### Защита от двойного завершения

`finish` внутри транзакции читает поездку с `SELECT ... FOR UPDATE`. Второй
параллельный запрос ждёт на блокировке строки. 

### Идемпотентность `POST /api/v1/trips`

Ключи хранятся в таблице `idempotency` (`key`, `body_hash`, `trip_id`,
`created_at`). 


Параллельный запрос с тем же ключом блокируется на уникальном ключе, пока первая
транзакция не завершится. Затем он читает запись: хеш совпал — `200` и та же
поездка, не совпал — `409 idempotency_conflict`. Если первая транзакция
откатилась, ключ свободен, и второй запрос создаёт поездку сам.

**Ключ живёт 1 час** (`IdempotencyTTL`). Просроченный ключ игнорируется при
чтении и удаляется при следующей попытке его занять, после чего его можно
использовать заново.

### Graceful shutdown

По `SIGINT` / `SIGTERM` сервер перестаёт принимать новые соединения и ждёт
активные запросы до `SHUTDOWN_TIMEOUT` (`http.Server.Shutdown`). Если за это
время не уложились, соединения закрываются принудительно. После этого
закрывается пул БД.
