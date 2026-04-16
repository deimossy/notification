package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strings"
	"time"

	"go.uber.org/zap"
)

const maxFailureReasonLength = 2048

type Service struct {
	log            *zap.Logger
	sender         Sender
	stateStore     MessageStateStore
	defaultSubject string
	lockTTL        time.Duration
}

func NewService(
	log *zap.Logger,
	sender Sender,
	stateStore MessageStateStore,
	defaultSubject string,
	lockTTL time.Duration,
) *Service {
	return &Service{
		log:            log,
		sender:         sender,
		stateStore:     stateStore,
		defaultSubject: defaultSubject,
		lockTTL:        lockTTL,
	}
}

func (s *Service) Process(ctx context.Context, payload []byte) error {
	var event SendEmailEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return NewPermanentError(fmt.Errorf("invalid payload format: %w", err))
	}

	if err := event.Validate(); err != nil {
		return NewPermanentError(fmt.Errorf("payload validation failed: %w", err))
	}

	state, err := s.stateStore.Acquire(ctx, event.MessageID, s.lockTTL)
	if err != nil {
		return fmt.Errorf("failed to acquire message state: %w", err)
	}

	log := s.log.With(
		zap.String("message_id", event.MessageID),
		zap.String("to", event.To),
		zap.String("correlation_id", event.CorrelationID),
	)

	switch state {
	case AcquireStateAlreadyProcessed:
		log.Info("skip already processed message")

		return nil
	case AcquireStateInProgress:
		log.Info("skip message that is currently being processed")

		return nil
	}

	message := event.ToMessage(s.defaultSubject)
	if err = s.sender.Send(ctx, message); err != nil {
		storeErr := s.stateStore.MarkFailed(ctx, event.MessageID, trimFailureReason(err.Error()))
		if storeErr != nil {
			log.Warn("failed to mark message as failed", zap.Error(storeErr))
		}

		if IsPermanentError(err) || !isRetryableError(err) {
			return NewPermanentError(fmt.Errorf("email delivery failed permanently: %w", err))
		}

		return fmt.Errorf("email delivery failed temporarily: %w", err)
	}

	if err = s.stateStore.MarkProcessed(ctx, event.MessageID); err != nil {
		return fmt.Errorf("failed to mark message as processed: %w", err)
	}

	log.Info("email delivered")

	return nil
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	if IsPermanentError(err) {
		return false
	}

	var tpError *textproto.Error
	if errors.As(err, &tpError) {
		return tpError.Code >= 400 && tpError.Code < 500
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	if errors.Is(err, io.EOF) {
		return true
	}

	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "timeout") ||
		strings.Contains(errText, "temporarily unavailable") ||
		strings.Contains(errText, "connection reset") ||
		strings.Contains(errText, "broken pipe")
}

func trimFailureReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) <= maxFailureReasonLength {
		return reason
	}

	return reason[:maxFailureReasonLength]
}
