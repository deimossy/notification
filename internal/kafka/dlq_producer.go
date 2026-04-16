package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type DLQProducer struct {
	writer *kafka.Writer
	topic  string
}

type DeadLetterMessage struct {
	OriginalTopic   string          `json:"original_topic"`
	OriginalKey     string          `json:"original_key,omitempty"`
	OriginalValue   json.RawMessage `json:"original_value"`
	Partition       int             `json:"partition"`
	Offset          int64           `json:"offset"`
	Error           string          `json:"error"`
	Attempts        int             `json:"attempts"`
	FailedAt        time.Time       `json:"failed_at"`
	CorrelationHint string          `json:"correlation_hint,omitempty"`
}

func NewDLQProducer(brokers []string, topic string) *DLQProducer {
	return &DLQProducer{
		topic: topic,
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			RequiredAcks: kafka.RequireAll,
			Async:        false,
			Balancer:     &kafka.LeastBytes{},
		},
	}
}

func (p *DLQProducer) Publish(ctx context.Context, msg kafka.Message, sourceTopic string, attempts int, processErr error) error {
	dlq := DeadLetterMessage{
		OriginalTopic: sourceTopic,
		OriginalKey:   string(msg.Key),
		OriginalValue: append(json.RawMessage(nil), msg.Value...),
		Partition:     msg.Partition,
		Offset:        msg.Offset,
		Error:         processErr.Error(),
		Attempts:      attempts,
		FailedAt:      time.Now().UTC(),
	}

	payload, err := json.Marshal(dlq)
	if err != nil {
		return fmt.Errorf("marshal dlq message: %w", err)
	}

	if err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   msg.Key,
		Value: payload,
		Time:  time.Now(),
	}); err != nil {
		return fmt.Errorf("write message to dlq topic %q: %w", p.topic, err)
	}

	return nil
}

func (p *DLQProducer) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}

	return p.writer.Close()
}
