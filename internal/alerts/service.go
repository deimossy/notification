package alerts

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/linspacestrom/go-project/internal/email"
	"go.uber.org/zap"
)

type Repository interface {
	InsertAlert(ctx context.Context, alert Alert) (bool, error)
	ListAlerts(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Alert, error)
	MarkRead(ctx context.Context, userID, alertID uuid.UUID) (bool, error)
	MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error)
	UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error)
}

type Service struct {
	log  *zap.Logger
	repo Repository
}

func NewService(log *zap.Logger, repo Repository) *Service {
	return &Service{log: log, repo: repo}
}

func (s *Service) Process(ctx context.Context, payload []byte) error {
	var event CreateAlertEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return email.NewPermanentError(fmt.Errorf("invalid alert event format: %w", err))
	}

	userID, err := event.Validate()
	if err != nil {
		return email.NewPermanentError(fmt.Errorf("invalid alert event payload: %w", err))
	}

	inserted, err := s.repo.InsertAlert(ctx, event.ToAlert(userID))
	if err != nil {
		return fmt.Errorf("insert alert: %w", err)
	}

	log := s.log.With(
		zap.String("event_id", event.EventID),
		zap.String("user_id", userID.String()),
		zap.String("type", event.Type),
	)

	if !inserted {
		log.Info("skip duplicate alert event")
		return nil
	}

	log.Info("alert event ingested")
	return nil
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Alert, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	items, err := s.repo.ListAlerts(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}

	return items, nil
}

func (s *Service) MarkRead(ctx context.Context, userID, alertID uuid.UUID) (bool, error) {
	updated, err := s.repo.MarkRead(ctx, userID, alertID)
	if err != nil {
		return false, fmt.Errorf("mark alert read: %w", err)
	}

	return updated, nil
}

func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	affected, err := s.repo.MarkAllRead(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all alerts read: %w", err)
	}

	return affected, nil
}

func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	count, err := s.repo.UnreadCount(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("get unread count: %w", err)
	}

	return count, nil
}
