package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/linspacestrom/go-project/internal/config"
)

type SMTPSender struct {
	address            string
	host               string
	from               string
	username           string
	password           string
	useTLS             bool
	startTLS           bool
	insecureSkipVerify bool
	dialTimeout        time.Duration
	sendTimeout        time.Duration
}

func NewSMTPSender(cfg config.SMTPConfig) *SMTPSender {
	return &SMTPSender{
		address:            cfg.Address(),
		host:               cfg.Host,
		from:               cfg.From,
		username:           cfg.Username,
		password:           cfg.Password,
		useTLS:             cfg.UseTLS,
		startTLS:           cfg.StartTLS,
		insecureSkipVerify: cfg.InsecureSkipVerify,
		dialTimeout:        cfg.DialTimeout,
		sendTimeout:        cfg.SendTimeout,
	}
}

func (s *SMTPSender) Send(ctx context.Context, message Message) error {
	if _, err := mail.ParseAddress(message.To); err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	if _, err := mail.ParseAddress(s.from); err != nil {
		return NewPermanentError(fmt.Errorf("invalid sender address in config: %w", err))
	}

	payload, err := buildMIMEMessage(s.from, message)
	if err != nil {
		return NewPermanentError(fmt.Errorf("failed to build email payload: %w", err))
	}

	dialer := &net.Dialer{Timeout: s.dialTimeout}
	ctx, cancel := context.WithTimeout(ctx, s.sendTimeout)
	defer cancel()

	var conn net.Conn
	if s.useTLS {
		tlsConn, tlsErr := tls.DialWithDialer(dialer, "tcp", s.address, &tls.Config{
			ServerName:         s.host,
			InsecureSkipVerify: s.insecureSkipVerify,
		})
		if tlsErr != nil {
			return fmt.Errorf("failed to establish SMTPS connection: %w", tlsErr)
		}

		conn = tlsConn
	} else {
		plainConn, dialErr := dialer.DialContext(ctx, "tcp", s.address)
		if dialErr != nil {
			return fmt.Errorf("failed to establish SMTP connection: %w", dialErr)
		}

		conn = plainConn
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		if setErr := conn.SetDeadline(deadline); setErr != nil {
			return fmt.Errorf("failed to set connection deadline: %w", setErr)
		}
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}

	shouldClose := true
	defer func() {
		if shouldClose {
			_ = client.Close()
		}
	}()

	if s.startTLS && !s.useTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return NewPermanentError(fmt.Errorf("SMTP server does not support STARTTLS"))
		}

		if err = client.StartTLS(&tls.Config{
			ServerName:         s.host,
			InsecureSkipVerify: s.insecureSkipVerify,
		}); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	if s.username != "" {
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err = client.Mail(s.from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}

	if err = client.Rcpt(message.To); err != nil {
		return fmt.Errorf("SMTP RCPT TO failed: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}

	if _, err = writer.Write(payload); err != nil {
		_ = writer.Close()

		return fmt.Errorf("failed to write email payload: %w", err)
	}

	if err = writer.Close(); err != nil {
		return fmt.Errorf("failed to finalize email payload: %w", err)
	}

	if err = client.Quit(); err != nil {
		return fmt.Errorf("failed to close SMTP session: %w", err)
	}

	shouldClose = false

	return nil
}

func buildMIMEMessage(from string, message Message) ([]byte, error) {
	toAddress, err := mail.ParseAddress(message.To)
	if err != nil {
		return nil, fmt.Errorf("parse recipient: %w", err)
	}

	subject := sanitizeHeaderValue(message.Subject)
	if subject == "" {
		subject = "Notification"
	}

	var payload bytes.Buffer
	payload.WriteString(fmt.Sprintf("From: %s\r\n", from))
	payload.WriteString(fmt.Sprintf("To: %s\r\n", toAddress.Address))
	payload.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	payload.WriteString("MIME-Version: 1.0\r\n")

	hasText := strings.TrimSpace(message.Text) != ""
	hasHTML := strings.TrimSpace(message.HTML) != ""

	switch {
	case hasText && hasHTML:
		boundary := "boundary_" + strconv.FormatInt(time.Now().UnixNano(), 10)
		payload.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
		payload.WriteString("\r\n")
		payload.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		payload.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		payload.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		payload.WriteString(message.Text)
		payload.WriteString("\r\n")
		payload.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		payload.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		payload.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		payload.WriteString(message.HTML)
		payload.WriteString("\r\n")
		payload.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	case hasHTML:
		payload.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		payload.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		payload.WriteString(message.HTML)
		payload.WriteString("\r\n")
	default:
		payload.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		payload.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
		payload.WriteString(message.Text)
		payload.WriteString("\r\n")
	}

	return payload.Bytes(), nil
}

func sanitizeHeaderValue(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")

	return strings.TrimSpace(value)
}
