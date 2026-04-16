package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/linspacestrom/go-project/internal/config"
	"github.com/linspacestrom/go-project/internal/email"
	"github.com/linspacestrom/go-project/internal/kafka"
	"github.com/linspacestrom/go-project/internal/repository"
	"github.com/linspacestrom/go-project/internal/server"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Repository interface {
	email.MessageStateStore
	Close()
}

type Consumer interface {
	Run(ctx context.Context) error
	Close() error
}

type App struct {
	log      *zap.Logger
	api      *server.Server
	cfg      *config.Config
	repo     Repository
	consumer Consumer
	cancel   context.CancelFunc
}

func New(log *zap.Logger, cfg *config.Config) (*App, error) {
	repo, trManager, err := repository.New(cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}
	_ = trManager

	sender := email.NewSMTPSender(cfg.SMTP)
	emailService := email.NewService(log, sender, repo, cfg.SMTP.DefaultSubject, cfg.Worker.LockTTL)
	consumer := kafka.NewConsumer(log, cfg.Kafka, cfg.Worker, emailService)
	api := server.New(log, cfg.Server)

	return &App{
		log:      log,
		api:      api,
		cfg:      cfg,
		repo:     repo,
		consumer: consumer,
	}, nil
}

func (a *App) Run() {
	defer func() {
		if r := recover(); r != nil {
			a.log.Error("application panicked", zap.Any("panic", r))
			a.Stop()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		if err := a.api.Run(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}

			return fmt.Errorf("http server failed: %w", err)
		}

		return nil
	})
	group.Go(func() error {
		if err := a.consumer.Run(groupCtx); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}

			return fmt.Errorf("kafka consumer failed: %w", err)
		}

		return nil
	})

	if err := group.Wait(); err != nil {
		a.log.Error("application terminated with error", zap.Error(err))
	}
}

func (a *App) Stop() {
	if a.cancel != nil {
		a.cancel()
	}

	a.log.Info("closing Kafka consumer")
	if err := a.consumer.Close(); err != nil {
		a.log.Error("failed to close Kafka consumer", zap.Error(err))
	}

	a.log.Info("closing HTTP server")
	if err := a.api.Close(); err != nil {
		a.log.Error("failed to close HTTP server", zap.Error(err))
	}

	a.log.Info("closing database connection pool")
	a.repo.Close()

	a.log.Info("application stopped gracefully")
}
