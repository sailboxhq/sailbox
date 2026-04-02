package ws

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/sailboxhq/sailbox/apps/api/internal/service"
)

type NodeLogsHandler struct {
	svc    *service.NodeService
	logger *slog.Logger
}

func NewNodeLogsHandler(svc *service.NodeService, logger *slog.Logger) *NodeLogsHandler {
	return &NodeLogsHandler{svc: svc, logger: logger}
}

func (h *NodeLogsHandler) Handle(c *gin.Context) {
	nodeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid node ID"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", slog.Any("error", err))
		return
	}
	defer func() { _ = conn.Close() }()

	// Read pump for disconnect detection
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ch := h.svc.SubscribeLogs(nodeID)
	defer h.svc.UnsubscribeLogs(nodeID, ch)

	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				return
			}
		}
	}
}
