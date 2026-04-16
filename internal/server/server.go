package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/linspacestrom/go-project/internal/config"
	"github.com/linspacestrom/go-project/internal/server/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Handler interface {
	RegisterRoutes(r gin.IRouter)
}

type Server struct {
	server *http.Server
	cfg    config.HTTPConfig
	router gin.IRouter
	log    *zap.Logger
}

func New(
	log *zap.Logger,
	cfg config.HTTPConfig,
	handlers ...Handler,
) *Server {
	router := registerRoutes(log, cfg, handlers...)

	return &Server{
		server: &http.Server{
			Addr:         cfg.GetAddr(),
			ReadTimeout:  cfg.Timeout.Read,
			WriteTimeout: cfg.Timeout.Write,
			IdleTimeout:  cfg.Timeout.Idle,
			Handler:      router,
		},
		router: router,
		cfg:    cfg,
		log:    log,
	}
}

func registerRoutes(log *zap.Logger, cfg config.HTTPConfig, handlers ...Handler) *gin.Engine {
	if cfg.Mode != "" {
		gin.SetMode(cfg.Mode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(log))
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	for _, handler := range handlers {
		handler.RegisterRoutes(r)
	}

	return r
}

func (s *Server) MustRun() {
	if err := s.Run(); err != nil {
		s.log.Panic("failed to run server", zap.Error(err))
	}
}

func (s *Server) Run() error {
	s.log.Info("starting server",
		zap.String("addr", s.server.Addr),
		zap.String("timeout", s.cfg.Timeout.Server.String()))

	err := s.server.ListenAndServe()

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Timeout.Server)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		s.log.Warn("failed to shutdown server", zap.Error(err))

		if closeErr := s.server.Close(); closeErr != nil {
			return fmt.Errorf("shutdown error: %w, close error: %w", err, closeErr)
		}

		return err
	}

	return nil
}
