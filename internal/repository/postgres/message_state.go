package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/linspacestrom/go-project/internal/email"
)

const (
	stateProcessing = "processing"
	stateProcessed  = "processed"
	stateFailed     = "failed"
)

func (r *Repository) Acquire(ctx context.Context, messageID string, lockTTL time.Duration) (email.AcquireState, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var status string
	var updatedAt time.Time

	err = tx.QueryRow(ctx, `
		SELECT status, updated_at
		FROM notification_email_message_state
		WHERE message_id = $1
		FOR UPDATE
	`, messageID).Scan(&status, &updatedAt)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if _, err = tx.Exec(ctx, `
			INSERT INTO notification_email_message_state (message_id, status, created_at, updated_at)
			VALUES ($1, $2, NOW(), NOW())
		`, messageID, stateProcessing); err != nil {
			return 0, fmt.Errorf("insert processing state: %w", err)
		}

		if err = tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("commit inserted state: %w", err)
		}

		return email.AcquireStateAcquired, nil
	case err != nil:
		return 0, fmt.Errorf("select state for update: %w", err)
	}

	now := time.Now().UTC()

	switch status {
	case stateProcessed:
		if err = tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("commit processed state read: %w", err)
		}

		return email.AcquireStateAlreadyProcessed, nil
	case stateFailed:
		if _, err = tx.Exec(ctx, `
			UPDATE notification_email_message_state
			SET status = $2,
				last_error = NULL,
				updated_at = NOW()
			WHERE message_id = $1
		`, messageID, stateProcessing); err != nil {
			return 0, fmt.Errorf("move failed state to processing: %w", err)
		}

		if err = tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("commit failed->processing update: %w", err)
		}

		return email.AcquireStateAcquired, nil
	case stateProcessing:
		if lockTTL > 0 && now.Sub(updatedAt) >= lockTTL {
			if _, err = tx.Exec(ctx, `
				UPDATE notification_email_message_state
				SET status = $2,
					last_error = NULL,
					updated_at = NOW()
				WHERE message_id = $1
			`, messageID, stateProcessing); err != nil {
				return 0, fmt.Errorf("re-acquire stale processing lock: %w", err)
			}

			if err = tx.Commit(ctx); err != nil {
				return 0, fmt.Errorf("commit stale lock reacquire: %w", err)
			}

			return email.AcquireStateAcquired, nil
		}

		if err = tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("commit processing state read: %w", err)
		}

		return email.AcquireStateInProgress, nil
	default:
		return 0, fmt.Errorf("unknown message state %q", status)
	}
}

func (r *Repository) MarkProcessed(ctx context.Context, messageID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO notification_email_message_state (message_id, status, last_error, created_at, updated_at)
		VALUES ($1, $2, NULL, NOW(), NOW())
		ON CONFLICT (message_id) DO UPDATE
		SET status = EXCLUDED.status,
			last_error = NULL,
			updated_at = NOW()
	`, messageID, stateProcessed)
	if err != nil {
		return fmt.Errorf("mark message as processed: %w", err)
	}

	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, messageID, failureReason string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO notification_email_message_state (message_id, status, last_error, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (message_id) DO UPDATE
		SET status = EXCLUDED.status,
			last_error = EXCLUDED.last_error,
			updated_at = NOW()
	`, messageID, stateFailed, failureReason)
	if err != nil {
		return fmt.Errorf("mark message as failed: %w", err)
	}

	return nil
}
