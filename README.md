# Notification Service

Микросервис для отправки email-уведомлений из Kafka.

## Поток обработки

1. Сервис читает сообщения из топика `kafka.topic` в consumer group `kafka.group_id`.
2. Валидирует payload (`message_id`, `to`, `text|html`).
3. Пытается захватить обработку сообщения через Postgres-таблицу `notification_email_message_state`.
4. Отправляет письмо через SMTP.
5. При успехе помечает сообщение как `processed` и коммитит offset.
6. При временных ошибках ретраит с exponential backoff.
7. При невалидном payload или исчерпании ретраев публикует событие в DLQ (`kafka.dlq_topic`) и коммитит offset.

## Формат входного Kafka-сообщения

```json
{
  "message_id": "31a5d5f6-b06a-4d34-8894-a8f8614a1a4f",
  "to": "user@example.com",
  "subject": "Welcome",
  "text": "Hello from notification service",
  "html": "<p>Hello from notification service</p>",
  "correlation_id": "req-42"
}
```

Поля:
- `message_id` — обязательный уникальный идентификатор сообщения.
- `to` — обязательный email получателя.
- `text`/`html` — должен быть минимум один из этих полей.
- `subject` — опционально, при отсутствии используется `smtp.default_subject`.

## Конфигурация

Основная конфигурация находится в `config/config.yaml`.
Чувствительные параметры (`SMTP_PASSWORD`, `POSTGRES_PASSWORD`) рекомендуется передавать через env.

## Миграции

```bash
go run ./cmd/migrator -command up
```

## Запуск

```bash
go run ./cmd/app
```

## Health endpoints

- `GET /healthz`
- `GET /readyz`

## Полный Локальный E2E Тест (Docker Compose)

### 1) Поднять инфраструктуру и сервис

```bash
docker compose up -d --build
```

Проверить, что все сервисы поднялись:

```bash
docker compose ps
docker compose logs -f notification
```

### 2) Проверить health

```bash
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
```

### 3) Отправить успешное сообщение в Kafka

```bash
docker compose exec -T kafka kafka-console-producer \
  --bootstrap-server kafka:9092 \
  --topic notifications.email.send <<'EOF'
{"message_id":"11111111-1111-1111-1111-111111111111","to":"user@example.com","subject":"Smoke test","text":"Hello from Kafka"}
EOF
```

### 4) Проверить успешную отправку

1. Открыть Mailpit UI: `http://localhost:8025` и убедиться, что письмо пришло.
2. Проверить состояние в Postgres:

```bash
docker compose exec -T postgres psql -U postgres -d notification -c \
"SELECT message_id,status,last_error,updated_at FROM notification_email_message_state ORDER BY updated_at DESC LIMIT 10;"
```

Для отправленного `message_id` статус должен быть `processed`.

### 5) Протестировать неуспешный кейс и DLQ

Отправить невалидный payload (без `to`):

```bash
docker compose exec -T kafka kafka-console-producer \
  --bootstrap-server kafka:9092 \
  --topic notifications.email.send <<'EOF'
{"message_id":"22222222-2222-2222-2222-222222222222","subject":"Bad event","text":"This must go to DLQ"}
EOF
```

Прочитать DLQ:

```bash
docker compose exec -T kafka kafka-console-consumer \
  --bootstrap-server kafka:9092 \
  --topic notifications.email.dlq \
  --from-beginning \
  --max-messages 1
```

Проверить состояние в БД:

```bash
docker compose exec -T postgres psql -U postgres -d notification -c \
"SELECT message_id,status,last_error,updated_at FROM notification_email_message_state WHERE message_id='22222222-2222-2222-2222-222222222222';"
```

Ожидаемый статус: `failed`.

### 6) Проверка идемпотентности

Отправить то же самое успешное сообщение второй раз (тот же `message_id`):

```bash
docker compose exec -T kafka kafka-console-producer \
  --bootstrap-server kafka:9092 \
  --topic notifications.email.send <<'EOF'
{"message_id":"11111111-1111-1111-1111-111111111111","to":"user@example.com","subject":"Duplicate","text":"Should be ignored as already processed"}
EOF
```

Сервис должен залогировать, что сообщение уже обработано, и не отправлять дубль.

### 7) Остановка и очистка

```bash
docker compose down
```

С удалением volume Postgres:

```bash
docker compose down -v
```
