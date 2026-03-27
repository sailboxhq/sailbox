package event

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

const channel = "sailbox_changes"

// PGSubscriber listens to PG NOTIFY on the sailbox_changes channel
// and exposes a Go channel of Change events.
type PGSubscriber struct {
	ln     *pgdriver.Listener
	ch     <-chan pgdriver.Notification
	out    chan Change
	logger *slog.Logger
}

// NewPGSubscriber creates a subscriber backed by pgdriver.Listener.
// It starts listening immediately in a background goroutine.
func NewPGSubscriber(db *bun.DB, logger *slog.Logger) (*PGSubscriber, error) {
	ln := pgdriver.NewListener(db)

	if err := ln.Listen(context.Background(), channel); err != nil {
		ln.Close()
		return nil, err
	}

	// pgdriver.Channel returns a buffered Go channel that handles reconnects internally.
	notifications := ln.Channel()
	out := make(chan Change, 128)

	s := &PGSubscriber{
		ln:     ln,
		ch:     notifications,
		out:    out,
		logger: logger,
	}

	go s.loop()
	return s, nil
}

// Changes returns the channel of parsed change events.
func (s *PGSubscriber) Changes() <-chan Change {
	return s.out
}

// Close stops the listener and closes the output channel.
func (s *PGSubscriber) Close() error {
	err := s.ln.Close()
	// out channel will be closed by the loop goroutine when ln.Channel() closes
	return err
}

func (s *PGSubscriber) loop() {
	defer close(s.out)

	for notification := range s.ch {
		var change Change
		if err := json.Unmarshal([]byte(notification.Payload), &change); err != nil {
			s.logger.Warn("invalid NOTIFY payload",
				slog.String("payload", notification.Payload),
				slog.Any("error", err),
			)
			continue
		}

		select {
		case s.out <- change:
		default:
			s.logger.Warn("change event dropped (slow consumer)")
		}
	}
}
