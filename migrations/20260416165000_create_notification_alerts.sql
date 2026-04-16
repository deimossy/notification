-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS notification_alerts (
    id UUID PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    user_id UUID NOT NULL,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    severity TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_read BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    read_at TIMESTAMPTZ,
    ingested_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_alerts_user_created_at
    ON notification_alerts (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_notification_alerts_user_unread
    ON notification_alerts (user_id, is_read);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_alerts;
-- +goose StatementEnd
