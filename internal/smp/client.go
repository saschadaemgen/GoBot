// Package smp implements the SMP frame-level client for connecting
// to SimpleX SMP servers. GoBot operates at the SMP frame level -
// it does not implement a full SimpleX chat client.
//
// GoBot knows SMP framing (16 KB blocks, queue IDs, subscriptions)
// but not message content. All crypto is handled by GoKey or by
// standalone mode.
package smp

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Block represents a raw encrypted SMP block received from a server.
type Block struct {
	// Server is the SMP server address (e.g. "smp15.simplex.im").
	Server string

	// QueueID identifies the SMP queue this block came from.
	QueueID string

	// Data is the raw encrypted block (up to 16 KB).
	Data []byte

	// ReceivedAt is when GoBot received this block.
	ReceivedAt time.Time
}

// QueueSubscription represents an active subscription to an SMP queue.
type QueueSubscription struct {
	Server  string
	QueueID string
	Active  bool
}

// ConnState represents the connection state to an SMP server.
type ConnState int

const (
	ConnDisconnected ConnState = iota
	ConnConnecting
	ConnConnected
	ConnReconnecting
)

// String returns a human-readable connection state.
func (s ConnState) String() string {
	switch s {
	case ConnDisconnected:
		return "disconnected"
	case ConnConnecting:
		return "connecting"
	case ConnConnected:
		return "connected"
	case ConnReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

// ServerConn tracks the connection state for a single SMP server.
type ServerConn struct {
	Server    string
	State     ConnState
	Queues    []string
	LastBlock time.Time
	Errors    int
}

// BlockHandler is called when GoBot receives an encrypted block
// from an SMP server. Implementations forward blocks to GoKey
// (production) or decrypt locally (standalone mode).
type BlockHandler func(ctx context.Context, block Block) error

// Manager manages connections to multiple SMP servers and
// delivers encrypted blocks via a BlockHandler.
type Manager struct {
	mu      sync.RWMutex
	log     *slog.Logger
	handler BlockHandler
	conns   map[string]*ServerConn
	blocks  chan Block
}

// NewManager creates a new SMP connection manager.
func NewManager(log *slog.Logger, handler BlockHandler) *Manager {
	return &Manager{
		log:     log,
		handler: handler,
		conns:   make(map[string]*ServerConn),
		blocks:  make(chan Block, 100),
	}
}

// Subscribe registers a queue on an SMP server. If no connection
// to the server exists, one is established first.
func (m *Manager) Subscribe(server, queueID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.conns[server]
	if !exists {
		conn = &ServerConn{
			Server: server,
			State:  ConnDisconnected,
			Queues: []string{},
		}
		m.conns[server] = conn
	}

	// Check for duplicate subscription.
	for _, q := range conn.Queues {
		if q == queueID {
			m.log.Debug("queue already subscribed",
				"server", server, "queue", queueID)
			return nil
		}
	}

	conn.Queues = append(conn.Queues, queueID)
	m.log.Info("queue subscribed",
		"server", server, "queue", queueID,
		"total_queues", len(conn.Queues))

	return nil
}

// Unsubscribe removes a queue subscription.
func (m *Manager) Unsubscribe(server, queueID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.conns[server]
	if !exists {
		return
	}

	for i, q := range conn.Queues {
		if q == queueID {
			conn.Queues = append(conn.Queues[:i], conn.Queues[i+1:]...)
			m.log.Info("queue unsubscribed",
				"server", server, "queue", queueID,
				"remaining_queues", len(conn.Queues))
			return
		}
	}
}

// Servers returns the current state of all server connections.
func (m *Manager) Servers() []ServerConn {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ServerConn, 0, len(m.conns))
	for _, conn := range m.conns {
		result = append(result, *conn)
	}
	return result
}

// Run starts the connection manager. It connects to all registered
// servers, subscribes to queues, and delivers blocks to the handler.
// Blocks until the context is cancelled.
//
// TODO(sprint2): Implement TLS connections to SMP servers.
// TODO(sprint2): Implement SMP frame parsing.
// TODO(sprint2): Implement reconnection with exponential backoff.
func (m *Manager) Run(ctx context.Context) error {
	m.log.Info("smp manager starting",
		"servers", len(m.conns))

	// Block dispatch loop.
	for {
		select {
		case <-ctx.Done():
			m.log.Info("smp manager stopping")
			return ctx.Err()
		case block := <-m.blocks:
			if err := m.handler(ctx, block); err != nil {
				m.log.Error("block handler failed",
					"server", block.Server,
					"queue", block.QueueID,
					"error", err)
			}
		}
	}
}
