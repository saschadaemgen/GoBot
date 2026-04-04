package smp

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}

func noopHandler(_ context.Context, _ Block) error {
	return nil
}

func TestNewManager(t *testing.T) {
	m := NewManager(testLogger(), noopHandler)
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}

	servers := m.Servers()
	if len(servers) != 0 {
		t.Errorf("Servers() = %d, want 0", len(servers))
	}
}

func TestSubscribe(t *testing.T) {
	m := NewManager(testLogger(), noopHandler)

	err := m.Subscribe("smp15.simplex.im", "queue_abc")
	if err != nil {
		t.Fatalf("Subscribe() error: %v", err)
	}

	servers := m.Servers()
	if len(servers) != 1 {
		t.Fatalf("Servers() = %d, want 1", len(servers))
	}
	if servers[0].Server != "smp15.simplex.im" {
		t.Errorf("Server = %q, want %q", servers[0].Server, "smp15.simplex.im")
	}
	if len(servers[0].Queues) != 1 {
		t.Errorf("Queues = %d, want 1", len(servers[0].Queues))
	}
}

func TestSubscribeDuplicate(t *testing.T) {
	m := NewManager(testLogger(), noopHandler)

	_ = m.Subscribe("smp15.simplex.im", "queue_abc")
	_ = m.Subscribe("smp15.simplex.im", "queue_abc")

	servers := m.Servers()
	if len(servers[0].Queues) != 1 {
		t.Errorf("Queues = %d, want 1 (duplicate should be ignored)", len(servers[0].Queues))
	}
}

func TestSubscribeMultipleQueues(t *testing.T) {
	m := NewManager(testLogger(), noopHandler)

	_ = m.Subscribe("smp15.simplex.im", "queue_abc")
	_ = m.Subscribe("smp15.simplex.im", "queue_def")
	_ = m.Subscribe("smp15.simplex.im", "queue_ghi")

	servers := m.Servers()
	if len(servers[0].Queues) != 3 {
		t.Errorf("Queues = %d, want 3", len(servers[0].Queues))
	}
}

func TestSubscribeMultipleServers(t *testing.T) {
	m := NewManager(testLogger(), noopHandler)

	_ = m.Subscribe("smp15.simplex.im", "queue_abc")
	_ = m.Subscribe("smp16.simplex.im", "queue_def")

	servers := m.Servers()
	if len(servers) != 2 {
		t.Errorf("Servers() = %d, want 2", len(servers))
	}
}

func TestUnsubscribe(t *testing.T) {
	m := NewManager(testLogger(), noopHandler)

	_ = m.Subscribe("smp15.simplex.im", "queue_abc")
	_ = m.Subscribe("smp15.simplex.im", "queue_def")

	m.Unsubscribe("smp15.simplex.im", "queue_abc")

	servers := m.Servers()
	if len(servers[0].Queues) != 1 {
		t.Errorf("Queues = %d, want 1 after unsubscribe", len(servers[0].Queues))
	}
	if servers[0].Queues[0] != "queue_def" {
		t.Errorf("remaining queue = %q, want %q", servers[0].Queues[0], "queue_def")
	}
}

func TestUnsubscribeNonexistent(t *testing.T) {
	m := NewManager(testLogger(), noopHandler)

	// Should not panic.
	m.Unsubscribe("smp15.simplex.im", "queue_abc")
}

func TestConnStateString(t *testing.T) {
	tests := []struct {
		state ConnState
		want  string
	}{
		{ConnDisconnected, "disconnected"},
		{ConnConnecting, "connecting"},
		{ConnConnected, "connected"},
		{ConnReconnecting, "reconnecting"},
		{ConnState(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("ConnState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
