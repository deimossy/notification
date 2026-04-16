package alerts

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/linspacestrom/go-project/internal/server/middleware"
	"go.uber.org/zap"
)

type Handler struct {
	service    *Service
	authSecret string
}

func NewHandler(service *Service, authSecret string) *Handler {
	return &Handler{service: service, authSecret: authSecret}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	group := r.Group("/api/v1/alerts")
	group.Use(middleware.JWTAuthMiddleware(h.authSecret))
	group.GET("", h.list)
	group.GET("/unread-count", h.unreadCount)
	group.PATCH("/:id/read", h.markRead)
	group.PATCH("/read-all", h.markAllRead)
}

func (h *Handler) list(c *gin.Context) {
	log := middleware.GetLoggerFromContext(c)
	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn("failed to get user id from context", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit := parseIntOrDefault(c.Query("limit"), 20)
	offset := parseIntOrDefault(c.Query("offset"), 0)

	items, err := h.service.List(c.Request.Context(), userID, limit, offset)
	if err != nil {
		log.Error("failed to list alerts", zap.Error(err), zap.String("user_id", userID.String()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list alerts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) unreadCount(c *gin.Context) {
	log := middleware.GetLoggerFromContext(c)
	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn("failed to get user id from context", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	count, err := h.service.UnreadCount(c.Request.Context(), userID)
	if err != nil {
		log.Error("failed to count unread alerts", zap.Error(err), zap.String("user_id", userID.String()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count unread alerts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *Handler) markRead(c *gin.Context) {
	log := middleware.GetLoggerFromContext(c)
	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn("failed to get user id from context", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	alertID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid alert id"})
		return
	}

	updated, err := h.service.MarkRead(c.Request.Context(), userID, alertID)
	if err != nil {
		log.Error("failed to mark alert as read", zap.Error(err), zap.String("user_id", userID.String()), zap.String("alert_id", alertID.String()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark alert as read"})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) markAllRead(c *gin.Context) {
	log := middleware.GetLoggerFromContext(c)
	userID, err := middleware.GetUserID(c)
	if err != nil {
		log.Warn("failed to get user id from context", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	affected, err := h.service.MarkAllRead(c.Request.Context(), userID)
	if err != nil {
		log.Error("failed to mark all alerts as read", zap.Error(err), zap.String("user_id", userID.String()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark all alerts as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"updated": affected})
}

func parseIntOrDefault(raw string, defaultValue int) int {
	if raw == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}

	return parsed
}
