package email

import (
	"errors"
	"net/mail"
	"strings"
)

type SendEmailEvent struct {
	MessageID     string `json:"message_id"`
	To            string `json:"to"`
	Subject       string `json:"subject,omitempty"`
	Text          string `json:"text,omitempty"`
	HTML          string `json:"html,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

type Message struct {
	MessageID     string
	To            string
	Subject       string
	Text          string
	HTML          string
	CorrelationID string
}

func (e *SendEmailEvent) Validate() error {
	e.To = strings.TrimSpace(e.To)
	e.MessageID = strings.TrimSpace(e.MessageID)
	e.Subject = strings.TrimSpace(e.Subject)
	e.Text = strings.TrimSpace(e.Text)
	e.HTML = strings.TrimSpace(e.HTML)

	if e.MessageID == "" {
		return errors.New("message_id is required")
	}

	if e.To == "" {
		return errors.New("to is required")
	}

	if _, err := mail.ParseAddress(e.To); err != nil {
		return errors.New("to is not a valid email")
	}

	if e.Text == "" && e.HTML == "" {
		return errors.New("either text or html body must be provided")
	}

	return nil
}

func (e SendEmailEvent) ToMessage(defaultSubject string) Message {
	subject := strings.TrimSpace(e.Subject)
	if subject == "" {
		subject = defaultSubject
	}

	return Message{
		MessageID:     e.MessageID,
		To:            e.To,
		Subject:       subject,
		Text:          e.Text,
		HTML:          e.HTML,
		CorrelationID: e.CorrelationID,
	}
}
