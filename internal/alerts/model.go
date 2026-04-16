package alerts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CreateAlertEvent struct {
	EventID   string          `json:"event_id"`
	UserID    string          `json:"user_id"`
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Message   string          `json:"message"`
	Severity  string          `json:"severity"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	CreatedAt *time.Time      `json:"created_at,omitempty"`
}

type Alert struct {
	ID        uuid.UUID       `json:"id"`
	EventID   string          `json:"event_id"`
	UserID    uuid.UUID       `json:"user_id"`
	Type      string          `json:"type"`
	Title     string          `json:"title"`
	Message   string          `json:"message"`
	Severity  string          `json:"severity"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	IsRead    bool            `json:"is_read"`
	CreatedAt time.Time       `json:"created_at"`
	ReadAt    *time.Time      `json:"read_at,omitempty"`
}

func (e *CreateAlertEvent) Validate() (uuid.UUID, error) {
	e.EventID = strings.TrimSpace(e.EventID)
	e.UserID = strings.TrimSpace(e.UserID)
	e.Type = strings.TrimSpace(e.Type)
	e.Title = strings.TrimSpace(e.Title)
	e.Message = strings.TrimSpace(e.Message)
	e.Severity = strings.TrimSpace(strings.ToLower(e.Severity))

	if e.EventID == "" {
		return uuid.Nil, errors.New("event_id is required")
	}
	if e.UserID == "" {
		return uuid.Nil, errors.New("user_id is required")
	}

	userID, err := uuid.Parse(e.UserID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("user_id is not a valid UUID: %w", err)
	}

	if e.Type == "" {
		return uuid.Nil, errors.New("type is required")
	}
	if e.Title == "" {
		return uuid.Nil, errors.New("title is required")
	}
	if e.Message == "" {
		return uuid.Nil, errors.New("message is required")
	}

	switch e.Severity {
	case "", "info", "warning", "critical", "success":
		if e.Severity == "" {
			e.Severity = "info"
		}
	default:
		return uuid.Nil, errors.New("severity must be one of: info, warning, critical, success")
	}

	if len(e.Payload) == 0 {
		e.Payload = json.RawMessage(`{}`)
	} else if !json.Valid(e.Payload) {
		return uuid.Nil, errors.New("payload is not valid JSON")
	}

	return userID, nil
}

func (e *CreateAlertEvent) ToAlert(userID uuid.UUID) Alert {
	createdAt := time.Now().UTC()
	if e.CreatedAt != nil {
		createdAt = e.CreatedAt.UTC()
	}

	return Alert{
		ID:        uuid.New(),
		EventID:   e.EventID,
		UserID:    userID,
		Type:      e.Type,
		Title:     e.Title,
		Message:   e.Message,
		Severity:  e.Severity,
		Payload:   append(json.RawMessage(nil), e.Payload...),
		CreatedAt: createdAt,
	}
}
