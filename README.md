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
| `make generate` | сгенерировать OpenAPI-код и моки через `go generate` |
| `make migrate` / `migrate-down` / `migrate-status` | применить / откатить последнюю / показать статус миграций |
| `make migrate-create NAME=...` | создать новую SQL-миграцию |
| `make format` / `make lint` | форматирование и линтеры |
| `make test` | запустить тесты с `-race -count=2` и показать итоговое покрытие |
| `make test-integration` | дважды проверить репозитории с реальным PostgreSQL и `-race` |

Для интеграционных тестов нужен PostgreSQL из `tripgoctl environment start` и
применённые миграции. `Makefile` читает `DATABASE_URL` из `.env`:

```bash
make migrate
make test-integration
```

Каждый тест создаёт записи со своими UUID и удаляет их после выполнения.

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
| `ENV` | нет | `local` | имя окружения в логах и телеметрии |
| `LOG_FORMAT` | нет | `json` | JSON в stdout; `pretty` включает PrettyHandler при `ENV=local` для ручного запуска |
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
| `OTEL_SERVICE_NAME` | нет | `trip-service` | имя сервиса в телеметрии |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | да | — | базовый адрес OTLP коллектора; его создаёт `tripgoctl` в `.env` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | нет | `http/protobuf` | транспорт экспорта: `http/protobuf` |
| `OTEL_TRACES_SAMPLER` | нет | `parentbased_always_on` | стандартная настройка сэмплирования OTel |
| `OTEL_RESOURCE_ATTRIBUTES` | нет | — | дополнительные атрибуты ресурса OTel |

Пример значений — в [`.env.example`](.env.example).

## Наблюдаемость

HTTP-запросы проходят через `Recover → Tracing → Logging → Metrics → роутер`.
Итоговый лог пишется один раз после обработки запроса; `http.route` и имя спана
содержат шаблон маршрута, поэтому UUID поездки не попадает в лейблы.
Ошибки `5xx` и паники отмечаются на спане; паника возвращает `500 internal_error`.
По умолчанию во всех окружениях логи пишутся в JSON в stdout. Для ручного
локального запуска можно задать `LOG_FORMAT=pretty`: существующий PrettyHandler
остаётся в `internal/lib/logger/handlers`.

`otelpgx` подключён к пулу PostgreSQL. Для SQL-запросов собираются спаны и
`db.client.operation.duration` с именем операции, таблицей и типом ошибки.
Счётчики `tripgo.trips.created` и `tripgo.trips.completed` увеличиваются после
успешной транзакции; повтор по ключу идемпотентности не увеличивает `created`.
Gauge `tripgo.trips.active` считывает количество активных поездок из PostgreSQL
во время сбора метрик.
Трейсы и метрики отправляются по OTLP; отсутствие коллектора не блокирует
обработку запросов.

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
├── cmd
│   └── service             # конфиг, запуск, сигнал и graceful shutdown
├── config                  # YAML-конфиг для локального запуска
├── contracts               # OpenAPI, AsyncAPI и protobuf-контракты
├── internal
│   ├── config              # загрузка конфигурации
│   ├── container           # создание и связывание зависимостей
│   ├── domain/trip         # сущность, переходы состояний и ошибки
│   ├── model               # данные между транспортом и юзкейсами
│   ├── usecase/trip
│   │   ├── create
│   │   ├── get
│   │   └── finish
│   ├── http
│   │   ├── trip/{create,get,finish}
│   │   ├── domainerror     # перевод ошибок в Problem
│   │   ├── middleware
│   │   ├── router          # адаптер к сгенерированному ServerInterface
│   │   ├── health
│   │   └── ready
│   ├── lib/logger/handlers  # локальный PrettyHandler для slog
│   ├── repository/trip     # SQL поездок, истории и идемпотентности
│   ├── infra
│   │   ├── postgres/transaction
│   │   └── telemetry
│   └── generated           # код из OpenAPI
├── migrations
```

Каждый сценарий и каждая ручка имеют свой пакет. Узкие интерфейсы зависимостей
объявлены в их `contract.go`.
У `health` нет внешних зависимостей, поэтому ему не нужен пустой контракт.
Преобразование результата в `api.Trip` общее для трёх ручек и находится в
`internal/http/trip/convert.go`; входной DTO создаётся в `http/trip/create`.

Зависимости идут от HTTP к юзкейсам, от юзкейсов к домену. Репозиторий и
транспорт не импортируют друг друга. Репозиторий реализует интерфейсы юзкейсов
структурно, без импорта их пакетов. Запреты импортов проверяет `depguard` в
`.golangci.yml`. OpenAPI уже содержит ручки позиций, но они пока возвращают
`501` через сгенерированный `Unimplemented`; таблицы позиций тоже пока нет.

## Решения

### Уровень изоляции — Read Committed

Транзакции нутри менеджера по умолчанию открываются с read commited. Инварианты сервиса на ограничениях схемы:

- запрет двух активных поездок — частичный уникальный индекс;
- защита от двойного завершения — `SELECT ... FOR UPDATE`.

Поэтому более строгие уровни (`REPEATABLE READ`, `SERIALIZABLE`) ничего бы не
добавили, но потребовали бы повторять транзакции при ошибках.

### Менеджер транзакций

`transaction.Manager` (`internal/infra/postgres/transaction/manager.go`) реализует
интерфейсы `txManager` юзкейсов с единственным методом
`Do(ctx, fn func(ctx) error) error`:

- `Do` открывает транзакцию через `pool.BeginTx`, кладёт `pgx.Tx` в `context`
  под приватным ключом и вызывает `fn` с этим контекстом;
- `fn` вернула `nil` — `COMMIT`; вернула ошибку — `ROLLBACK` и ошибка
  пробрасывается наружу; паника — `ROLLBACK` и паника пробрасывается дальше;
- если в контексте уже есть транзакция (вложенный `Do`), `fn` выполняется в ней,
  вторая транзакция не открывается;
- репозиторий получает исполнителя через `transaction.Executor(ctx, pool)`: транзакция из
  контекста, если она есть, иначе пул. Транзакция не передаётся аргументом
  метода, поэтому юзкейсы не знают про `pgx`. Вызов репозитория вне `Do`
  просто выполняется на пуле в автокоммите.

Создание поездки выполняется в одном `Do`: резервирование ключа идемпотентности,
`INSERT` в `trips`, `INSERT` в `trip_status_history` и связывание ключа с поездкой.
Если какая-либо запись падает, откатываются все.
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

**Ключ живёт 1 час** (`idempotencyTTL` в `usecase/trip/create`). Просроченный ключ игнорируется при
чтении и удаляется при следующей попытке его занять, после чего его можно
использовать заново.

### Graceful shutdown

По `SIGINT` / `SIGTERM` сервер перестаёт принимать новые соединения и ждёт
активные запросы до `SHUTDOWN_TIMEOUT` (`http.Server.Shutdown`). Если за это
время не уложились, соединения закрываются принудительно. Затем останавливаются
фоновые компоненты и провайдеры OpenTelemetry, отправляя накопленные трейсы и
метрики; после них закрывается пул БД. Все эти действия используют один общий
бюджет `SHUTDOWN_TIMEOUT`.
Недоступность коллектора не препятствует запуску и обработке запросов.
