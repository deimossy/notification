-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS notification_email_message_state (
    message_id TEXT PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('processing', 'processed', 'failed')),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_email_message_state_status
    ON notification_email_message_state (status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_email_message_state;
-- +goose StatementEnd
