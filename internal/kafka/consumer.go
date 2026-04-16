package kafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/linspacestrom/go-project/internal/config"
	"github.com/linspacestrom/go-project/internal/email"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Processor interface {
	Process(ctx context.Context, payload []byte) error
}

type Consumer struct {
	log         *zap.Logger
	reader      *kafka.Reader
	dlqProducer *DLQProducer
	processor   Processor
	kafkaCfg    config.KafkaConfig
	workerCfg   config.WorkerConfig
}

func NewConsumer(
	log *zap.Logger,
	kafkaCfg config.KafkaConfig,
	workerCfg config.WorkerConfig,
	processor Processor,
) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:                kafkaCfg.Brokers,
		Topic:                  kafkaCfg.Topic,
		GroupID:                kafkaCfg.GroupID,
		MinBytes:               kafkaCfg.ReadMinBytes,
		MaxBytes:               kafkaCfg.ReadMaxBytes,
		CommitInterval:         0,
		WatchPartitionChanges:  true,
		PartitionWatchInterval: 5 * time.Second,
	})

	return &Consumer{
		log:         log,
		reader:      reader,
		dlqProducer: NewDLQProducer(kafkaCfg.Brokers, kafkaCfg.DLQTopic),
		processor:   processor,
		kafkaCfg:    kafkaCfg,
		workerCfg:   workerCfg,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("starting kafka consumer",
		zap.Strings("brokers", c.kafkaCfg.Brokers),
		zap.String("topic", c.kafkaCfg.Topic),
		zap.String("dlq_topic", c.kafkaCfg.DLQTopic),
		zap.String("group_id", c.kafkaCfg.GroupID),
	)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}

			c.log.Error("failed to fetch kafka message", zap.Error(err))
			if waitErr := c.wait(ctx, c.workerCfg.LoopErrorBackoff); waitErr != nil {
				return nil
			}

			continue
		}

		if err = c.processAndCommit(ctx, msg); err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}

			c.log.Error("failed to process kafka message", zap.Error(err),
				zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
			)
		}
	}
}

func (c *Consumer) processAndCommit(ctx context.Context, msg kafka.Message) error {
	attemptLimit := c.workerCfg.MaxRetries + 1
	if attemptLimit < 1 {
		attemptLimit = 1
	}

	attemptsDone := 0
	var processErr error

	for attempt := 1; attempt <= attemptLimit; attempt++ {
		attemptsDone = attempt

		processCtx := ctx
		cancel := func() {}
		if c.workerCfg.ProcessingTimeout > 0 {
			processCtx, cancel = context.WithTimeout(ctx, c.workerCfg.ProcessingTimeout)
		}

		processErr = c.processor.Process(processCtx, msg.Value)
		cancel()

		if processErr == nil {
			return c.commit(ctx, msg)
		}

		if errors.Is(processErr, context.Canceled) || ctx.Err() != nil {
			return context.Canceled
		}

		if email.IsPermanentError(processErr) {
			break
		}

		if attempt < attemptLimit {
			backoff := c.retryBackoff(attempt)
			c.log.Warn("retrying kafka message processing",
				zap.Int("attempt", attempt),
				zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Duration("backoff", backoff),
				zap.Error(processErr),
			)

			if err := c.wait(ctx, backoff); err != nil {
				return context.Canceled
			}
		}
	}

	if processErr == nil {
		processErr = errors.New("unknown processing error")
	}

	if err := c.dlqProducer.Publish(ctx, msg, c.kafkaCfg.Topic, attemptsDone, processErr); err != nil {
		return fmt.Errorf("failed to publish message to DLQ: %w", err)
	}

	c.log.Error("message moved to DLQ",
		zap.Int("partition", msg.Partition),
		zap.Int64("offset", msg.Offset),
		zap.Int("attempts", attemptsDone),
		zap.Error(processErr),
	)

	return c.commit(ctx, msg)
}

func (c *Consumer) commit(ctx context.Context, msg kafka.Message) error {
	commitCtx := ctx
	cancel := func() {}

	if c.kafkaCfg.CommitTimeout > 0 {
		commitCtx, cancel = context.WithTimeout(ctx, c.kafkaCfg.CommitTimeout)
	}
	defer cancel()

	if err := c.reader.CommitMessages(commitCtx, msg); err != nil {
		return fmt.Errorf("failed to commit kafka offset: %w", err)
	}

	return nil
}

func (c *Consumer) Close() error {
	var result error

	if c.reader != nil {
		if err := c.reader.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close kafka reader: %w", err))
		}
	}

	if c.dlqProducer != nil {
		if err := c.dlqProducer.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close dlq producer: %w", err))
		}
	}

	return result
}

func (c *Consumer) retryBackoff(attempt int) time.Duration {
	if c.workerCfg.InitialBackoff <= 0 {
		return 0
	}

	backoff := c.workerCfg.InitialBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if c.workerCfg.MaxBackoff > 0 && backoff >= c.workerCfg.MaxBackoff {
			return c.workerCfg.MaxBackoff
		}
	}

	if c.workerCfg.MaxBackoff > 0 && backoff > c.workerCfg.MaxBackoff {
		return c.workerCfg.MaxBackoff
	}

	return backoff
}

func (c *Consumer) wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
