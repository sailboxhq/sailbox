package event

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

const channel = "sailbox_changes"

// PGSubscriber listens to PG NOTIFY on the sailbox_changes channel
// and exposes a Go channel of Change events.
// It automatically reconnects if the database connection is lost.
type PGSubscriber struct {
	db     *bun.DB
	out    chan Change
	logger *slog.Logger
	done   chan struct{}
}

// NewPGSubscriber creates a subscriber that auto-reconnects on failure.
func NewPGSubscriber(db *bun.DB, logger *slog.Logger) (*PGSubscriber, error) {
	s := &PGSubscriber{
		db:     db,
		out:    make(chan Change, 128),
		logger: logger,
		done:   make(chan struct{}),
	}

	go s.connectLoop()
	return s, nil
}

// Changes returns the channel of parsed change events.
func (s *PGSubscriber) Changes() <-chan Change {
	return s.out
}

// Close stops the subscriber.
func (s *PGSubscriber) Close() error {
	close(s.done)
	return nil
}

func (s *PGSubscriber) connectLoop() {
	defer close(s.out)

	backoff := time.Second
	for {
		select {
		case <-s.done:
			return
		default:
		}

		if err := s.listen(); err != nil {
			s.logger.Warn("PG listener disconnected, reconnecting...", slog.Any("error", err), slog.Duration("backoff", backoff))
		}

		select {
		case <-s.done:
			return
		case <-time.After(backoff):
		}

		// Exponential backoff: 1s, 2s, 4s, 8s, max 30s
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

func (s *PGSubscriber) listen() error {
	ln := pgdriver.NewListener(s.db)
	defer func() { _ = ln.Close() }()

	if err := ln.Listen(context.Background(), channel); err != nil {
		return err
	}

	s.logger.Info("PG NOTIFY listener connected")

	notifications := ln.Channel()
	for {
		select {
		case <-s.done:
			return nil
		case notification, ok := <-notifications:
			if !ok {
				return nil // channel closed, reconnect
			}
			var change Change
			if err := json.Unmarshal([]byte(notification.Payload), &change); err != nil {
				s.logger.Warn("invalid NOTIFY payload", slog.String("payload", notification.Payload), slog.Any("error", err))
				continue
			}
			select {
			case s.out <- change:
			default:
				s.logger.Warn("change event dropped (slow consumer)")
			}
		}
	}
}
