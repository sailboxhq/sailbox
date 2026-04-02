package ws

import (
	"bufio"
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	"github.com/sailboxhq/sailbox/apps/api/internal/model"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type LogsHandler struct {
	store  store.Store
	orch   orchestrator.Orchestrator
	logger *slog.Logger
}

func NewLogsHandler(s store.Store, orch orchestrator.Orchestrator, logger *slog.Logger) *LogsHandler {
	return &LogsHandler{store: s, orch: orch, logger: logger}
}

func (h *LogsHandler) Handle(c *gin.Context) {
	appID, err := uuid.Parse(c.Param("appId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid app ID"})
		return
	}

	app, err := h.store.Applications().GetByID(c.Request.Context(), appID)
	if err != nil {
		c.JSON(404, gin.H{"error": "app not found"})
		return
	}

	// Verify app belongs to the authenticated user's org
	project, err := h.store.Projects().GetByID(c.Request.Context(), app.ProjectID)
	if err != nil || project.OrgID != middleware.GetOrgID(c) {
		c.JSON(403, gin.H{"error": "access denied"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", slog.Any("error", err))
		return
	}
	defer func() { _ = conn.Close() }()

	podName := c.Query("pod")
	timestamps := c.Query("timestamps") == "true"

	h.streamLogs(conn, app, podName, timestamps)
}

func (h *LogsHandler) streamLogs(conn *websocket.Conn, app *model.Application, podName string, timestamps bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Read pump: detect client disconnect
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	opts := orchestrator.LogOpts{
		Follow:     true,
		TailLines:  100,
		Timestamps: timestamps,
	}

	var reader interface {
		Read(p []byte) (n int, err error)
		Close() error
	}
	var err error

	if podName != "" {
		reader, err = h.orch.StreamPodLogs(ctx, app, podName, opts)
	} else {
		reader, err = h.orch.StreamLogs(ctx, app, opts)
	}
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()))
		return
	}
	defer func() { _ = reader.Close() }()

	h.logger.Info("log stream started", slog.String("app", app.Name))

	// Stream logs in a goroutine
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				return
			}
		}
	}()

	// Wait for either stream end or client disconnect
	select {
	case <-done:
		// Stream ended, keep alive with heartbeats until client disconnects
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	case <-ctx.Done():
		return
	}
}
