# Notification Service

Микросервис уведомлений:
- отправка email из Kafka,
- inbox-алерты (события из Kafka -> сохранение в БД -> чтение фронтом по API).

## Потоки обработки

### 1) Email pipeline
1. Сервис читает события из `kafka.topic`.
2. Валидирует payload (`message_id`, `to`, `text|html`).
3. Отправляет письмо через SMTP.
4. Пишет результат в `notification_email_message_state`.
5. При невосстановимых ошибках отправляет событие в `kafka.dlq_topic`.

### 2) Alerts inbox pipeline
1. Сервис читает события из `alerts_kafka.topic` (например, `notifications.alerts.created`).
2. Валидирует payload алерта (`event_id`, `user_id`, `title`, `message` и т.д.).
3. Сохраняет алерт в `notification_alerts`.
4. По `event_id` выполняется идемпотентность (`UNIQUE`) — дубль не создается.
5. Невалидные/фатальные события отправляются в `alerts_kafka.dlq_topic`.

## Формат входного события для алертов

```json
{
  "event_id": "event-001",
  "user_id": "11111111-1111-1111-1111-111111111111",
  "type": "invoice",
  "title": "Новый счет",
  "message": "Счет #123 готов к оплате",
  "severity": "info",
  "payload": {
    "invoice_id": "123"
  },
  "created_at": "2026-04-16T15:30:00Z"
}
```

`severity`: `info | warning | critical | success`.

## API для фронта (alerts)

Все эндпоинты требуют `Authorization: Bearer <JWT>`.
`user_id` берется из JWT claim `user_id`.

- `GET /api/v1/alerts?limit=20&offset=0`
- `GET /api/v1/alerts/unread-count`
- `PATCH /api/v1/alerts/:id/read`
- `PATCH /api/v1/alerts/read-all`

## Конфигурация

Основная конфигурация — `config/config.yaml`.
Отдельный блок Kafka для алертов: `alerts_kafka`.

## Миграции

```bash
go run ./cmd/migrator -command up
```

## Запуск

```bash
go run ./cmd/app
```

## Health

- `GET /healthz`
- `GET /readyz`

## Docker Compose

Поднимает: Postgres, Kafka (KRaft), Kafka topic init, Mailpit, migrator, notification.

```bash
docker compose up -d --build
```

Проверить:

```bash
docker compose ps
docker compose logs -f notification
```

Остановить:

```bash
docker compose down
```
