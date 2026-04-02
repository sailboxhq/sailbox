package ws

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sailboxhq/sailbox/apps/api/internal/event"
)

// SSEBroker manages SSE client connections and broadcasts
// database change events from PG LISTEN/NOTIFY.
type SSEBroker struct {
	mu      sync.RWMutex
	clients map[chan event.Change]struct{}
	logger  *slog.Logger
}

func NewSSEBroker(logger *slog.Logger) *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan event.Change]struct{}),
		logger:  logger,
	}
}

// Run reads from the subscriber and fans out to all connected SSE clients.
// Blocks until the subscriber channel closes — run in a goroutine.
func (b *SSEBroker) Run(sub event.Subscriber) {
	for change := range sub.Changes() {
		b.mu.RLock()
		for ch := range b.clients {
			select {
			case ch <- change:
			default:
				// skip slow client
			}
		}
		b.mu.RUnlock()
	}
	b.logger.Info("SSE broker: subscriber closed")
}

func (b *SSEBroker) subscribe() chan event.Change {
	ch := make(chan event.Change, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *SSEBroker) unsubscribe(ch chan event.Change) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

// ServeHTTP is the Gin handler for GET /ws/events (SSE).
func (b *SSEBroker) ServeHTTP(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // disable nginx/proxy buffering
	c.Writer.Flush()

	ch := b.subscribe()
	defer b.unsubscribe(ch)

	ctx := c.Request.Context()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case change := <-ch:
			data, _ := json.Marshal(change)
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			c.Writer.Flush()
		case <-heartbeat.C:
			_, _ = fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			c.Writer.Flush()
		}
	}
}
