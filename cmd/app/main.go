package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/linspacestrom/go-project/internal/app"
	"github.com/linspacestrom/go-project/internal/config"
	"github.com/linspacestrom/go-project/internal/logger"
	"go.uber.org/zap"
)

func main() {
	cfg := config.MustLoad()

	log := logger.MustLoad(cfg.Logger.Path)
	defer log.Sync()

	application, err := app.New(log, cfg)
	if err != nil {
		log.Fatal("failed to initialize application", zap.Error(err))
	}

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		application.Run()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Info("receiving shutdown signal", zap.String("signal", sig.String()))
	case <-runDone:
		log.Warn("application run loop exited")
	}

	application.Stop()
}
