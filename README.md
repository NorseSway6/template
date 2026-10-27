# TripGO

TripGo — MVP сервиса такси. Пассажир выбирает водителя и создаёт поездку. Пока поездка активна, устройство водителя шлёт координаты, чтобы потом можно было посмотреть маршрут. В конце водитель завершает поездку.

## Запуск

Запустить проект можно одним из двух способов.

Для обоих случаев нужна установленная утилита tripgoctl и запущенный кластер:

```bash
tripgoctl cluster start
```

Запуск с помощью tripgoctl:

```bash
make env-start
make env-connect
```

Запуск с помощью docker:

```bash
make docker-build
make docker-run
```

Вес собираемого образа: 4.68MB.

## Переменные env

```bash
HTTP_ADDR=:8080
LOG_LEVEL=info
SHUTDOWN_TIMEOUT=10s
DATABASE_URL=postgres://tripgo:tripgo@localhost:21032/tripgo?sslmode=disable
DATABASE_MAX_CONNS=10
DATABASE_MIN_CONNS=2
DATABASE_MAX_CONN_LIFETIME=30m
DATABASE_CONNECT_TIMEOUT=5s
DATABASE_QUERY_TIMEOUT=3s
OTEL_SERVICE_NAME=trip-service
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:22318
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_RESOURCE_ATTRIBUTES=service.namespace=tripgo,deployment.environment=local
OTEL_TRACES_SAMPLER=parentbased_always_on
HTTP_READ_HEADER_TIMEOUT=2s
HTTP_READ_TIMEOUT=5s
HTTP_WRITE_TIMEOUT=10s
HTTP_IDLE_TIMEOUT=30s
```

## Тестирование

```bash
make test
make test-integration
```

## Сделано

**ЛР1:**

- Создана структура репозитория
- Сделана конфигурация из окружения
- Поднят HTTP-сервер на net/http с роутером chi
- Сгенерирован код по OpenAPI
- Написаны миграции
- Подключен PostgreSQL через pgxpool
- Реализован репозитории на pgx + squirrel
- Реализован менеджер транзакций
- Реализовать три ручки по контракту + служебные ручки
- Реализована идемпотентноть (\*)
- Написан Dockerfile с запуском рядом с БД.

**ЛР2:**

- Код разложен по слоям
- Введены доменные модели и ошибки
- Введены интерфейсы
- Сделан единый маппинг ошибок
- Настроен линтер
- Написаны юнит тесты с моками и интеграционные тесты
- Подключена телеметрия

## Решения

Используется уровень изоляции Read Committed, так как он дает хорошую производительность и обеспечивает защиту на уровне СУБД.

Менеджер транзакций реализован через замыкание и контекст, что позволяет управять границами транзакциями, не завися от конкретных библиотек работы с БД.

Запрет двух активных поездок у одного водителя реализован на уровне СУБД с помощью частичного уникального индекса.

## Архитектура

```text
cmd/
  service/main.go             читает конфиг, собирает контейнер, запускает, ждёт сигнала
internal/
  config/                     разбор и валидация env
  container/                  создаёт все зависимости и связывает их между собой
  domain/                     доменные правила, ничего не знает про инфраструктуру
    trip/                     поездка: статусы и переходы, правила, ошибки домена
  model/                      структуры, которые ходят между слоями
  usecase/
    trip/
      create/                 usecase.go, contract.go, usecase_test.go, mocks_test.go
      get/
      finish/
  http/                       транспорт, пакет на ручку, server.go, contract.go
    trip/
      create/                 handler.go, contract.go, convert.go, mocks_test.go
      get/
      finish/
    domainerror/              маппинг доменной ошибки в HTTP-ответ, одно место
    middleware/
  repository/
    trip/                     repo.go, save.go, fetch.go, convert.go, contract.go
  infra/
    postgres/                 пул pgx
      transaction/            менеджер транзакций
    telemetry/                инициализация OpenTelemetry и slog
  generated/                  сгенерированный по OpenAPI код
migrations/
```

## Дополнительно

Реализована идемпотентность по заголовку Idempotency-Key.

Написан Dockerfile с многостадийной сборкой и запуском сервиса в окружении рядом с PostgreSQL с помощью команд из Makefile.
