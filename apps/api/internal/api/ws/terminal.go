package ws

import (
	"encoding/json"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/sailboxhq/sailbox/apps/api/internal/api/middleware"
	"github.com/sailboxhq/sailbox/apps/api/internal/orchestrator"
	"github.com/sailboxhq/sailbox/apps/api/internal/store"
)

type TerminalHandler struct {
	store  store.Store
	orch   orchestrator.Orchestrator
	logger *slog.Logger
}

func NewTerminalHandler(s store.Store, orch orchestrator.Orchestrator, logger *slog.Logger) *TerminalHandler {
	return &TerminalHandler{store: s, orch: orch, logger: logger}
}

func (h *TerminalHandler) Handle(c *gin.Context) {
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

	opts := orchestrator.ExecOpts{
		Command: []string{"/bin/sh"},
		TTY:     true,
	}

	session, err := h.orch.ExecTerminal(c.Request.Context(), app, opts)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Error: "+err.Error()+"\r\n"))
		return
	}
	defer func() { _ = session.Close() }()

	h.logger.Info("terminal session started", slog.String("app", app.Name))

	// stdout -> WebSocket (server -> browser)
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := session.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// WebSocket -> stdin (browser -> server)
	go func() {
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				_ = session.Close()
				return
			}

			if msgType == websocket.TextMessage {
				// Check for resize message
				var resize struct {
					Type string `json:"type"`
					Cols uint16 `json:"cols"`
					Rows uint16 `json:"rows"`
				}
				if json.Unmarshal(msg, &resize) == nil && resize.Type == "resize" {
					_ = session.Resize(resize.Cols, resize.Rows)
					continue
				}

				// Regular input
				_, _ = session.Write(msg)
			}
		}
	}()

	<-done
	h.logger.Info("terminal session ended", slog.String("app", app.Name))
}
