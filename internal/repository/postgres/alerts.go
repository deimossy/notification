package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/linspacestrom/go-project/internal/alerts"
)

func (r *Repository) InsertAlert(ctx context.Context, alert alerts.Alert) (bool, error) {
	result, err := r.db.Exec(ctx, `
		INSERT INTO notification_alerts (
			id,
			event_id,
			user_id,
			type,
			title,
			message,
			severity,
			payload,
			is_read,
			created_at,
			read_at,
			ingested_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,false,$9,NULL,NOW())
		ON CONFLICT (event_id) DO NOTHING
	`, alert.ID, alert.EventID, alert.UserID, alert.Type, alert.Title, alert.Message, alert.Severity, alert.Payload, alert.CreatedAt)
	if err != nil {
		return false, fmt.Errorf("insert alert: %w", err)
	}

	return result.RowsAffected() > 0, nil
}

func (r *Repository) ListAlerts(ctx context.Context, userID uuid.UUID, limit, offset int) ([]alerts.Alert, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, event_id, user_id, type, title, message, severity, payload, is_read, created_at, read_at
		FROM notification_alerts
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query alerts list: %w", err)
	}
	defer rows.Close()

	result := make([]alerts.Alert, 0, limit)
	for rows.Next() {
		var item alerts.Alert
		var payload []byte
		if err = rows.Scan(
			&item.ID,
			&item.EventID,
			&item.UserID,
			&item.Type,
			&item.Title,
			&item.Message,
			&item.Severity,
			&payload,
			&item.IsRead,
			&item.CreatedAt,
			&item.ReadAt,
		); err != nil {
			return nil, fmt.Errorf("scan alert row: %w", err)
		}

		if len(payload) == 0 {
			item.Payload = json.RawMessage(`{}`)
		} else {
			item.Payload = append(json.RawMessage(nil), payload...)
		}

		result = append(result, item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alerts list rows: %w", err)
	}

	return result, nil
}

func (r *Repository) MarkRead(ctx context.Context, userID, alertID uuid.UUID) (bool, error) {
	result, err := r.db.Exec(ctx, `
		UPDATE notification_alerts
		SET is_read = true,
			read_at = COALESCE(read_at, NOW())
		WHERE id = $1
		  AND user_id = $2
	`, alertID, userID)
	if err != nil {
		return false, fmt.Errorf("mark alert as read: %w", err)
	}

	return result.RowsAffected() > 0, nil
}

func (r *Repository) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	result, err := r.db.Exec(ctx, `
		UPDATE notification_alerts
		SET is_read = true,
			read_at = COALESCE(read_at, NOW())
		WHERE user_id = $1
		  AND is_read = false
	`, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all alerts as read: %w", err)
	}

	return result.RowsAffected(), nil
}

func (r *Repository) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM notification_alerts
		WHERE user_id = $1
		  AND is_read = false
	`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread alerts: %w", err)
	}

	return count, nil
}
