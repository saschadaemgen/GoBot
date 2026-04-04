package proxy

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/saschadaemgen/gobot/internal/protocol"
	"github.com/saschadaemgen/gobot/internal/smp"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}

func TestNewProxy(t *testing.T) {
	p := New(testLogger(), ModeStandalone, nil)
	if p == nil {
		t.Fatal("New() returned nil")
	}
	if p.Mode() != ModeStandalone {
		t.Errorf("Mode() = %v, want standalone", p.Mode())
	}
}

func TestModeString(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModeStandalone, "standalone"},
		{ModeGoKey, "gokey"},
		{Mode(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("Mode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestHandleBlock(t *testing.T) {
	p := New(testLogger(), ModeStandalone, nil)

	block := smp.Block{
		Server:     "smp15.simplex.im",
		QueueID:    "abc123",
		Data:       []byte("test encrypted data"),
		ReceivedAt: time.Now(),
	}

	err := p.HandleBlock(context.Background(), block)
	if err != nil {
		t.Fatalf("HandleBlock() error: %v", err)
	}

	stats := p.Stats()
	if stats.BlocksReceived != 1 {
		t.Errorf("BlocksReceived = %d, want 1", stats.BlocksReceived)
	}
	if stats.BlocksForwarded != 1 {
		t.Errorf("BlocksForwarded = %d, want 1", stats.BlocksForwarded)
	}
	if stats.Errors != 0 {
		t.Errorf("Errors = %d, want 0", stats.Errors)
	}
}

func TestHandleMultipleBlocks(t *testing.T) {
	p := New(testLogger(), ModeGoKey, nil)

	for i := 0; i < 10; i++ {
		block := smp.Block{
			Server:     "smp15.simplex.im",
			QueueID:    "abc123",
			Data:       []byte("encrypted data"),
			ReceivedAt: time.Now(),
		}
		if err := p.HandleBlock(context.Background(), block); err != nil {
			t.Fatalf("HandleBlock() #%d error: %v", i, err)
		}
	}

	stats := p.Stats()
	if stats.BlocksReceived != 10 {
		t.Errorf("BlocksReceived = %d, want 10", stats.BlocksReceived)
	}
	if stats.BlocksForwarded != 10 {
		t.Errorf("BlocksForwarded = %d, want 10", stats.BlocksForwarded)
	}
}

func TestHandleResultDummy(t *testing.T) {
	called := false
	handler := func(_ context.Context, _ protocol.ResultMsg) error {
		called = true
		return nil
	}

	p := New(testLogger(), ModeGoKey, handler)

	dummy := protocol.ResultMsg{
		V:         protocol.ProtocolVersion,
		Type:      protocol.TypeResult,
		HasAction: false,
		Action: protocol.Action{
			Command: protocol.CmdNone,
		},
	}

	err := p.HandleResult(context.Background(), dummy)
	if err != nil {
		t.Fatalf("HandleResult() error: %v", err)
	}

	if called {
		t.Error("handler should NOT be called for dummy results")
	}

	stats := p.Stats()
	if stats.ResultsReceived != 1 {
		t.Errorf("ResultsReceived = %d, want 1", stats.ResultsReceived)
	}
	if stats.CommandsExec != 0 {
		t.Errorf("CommandsExec = %d, want 0 for dummy", stats.CommandsExec)
	}
}

func TestHandleResultReal(t *testing.T) {
	var receivedCmd string
	handler := func(_ context.Context, result protocol.ResultMsg) error {
		receivedCmd = result.Action.Command
		return nil
	}

	p := New(testLogger(), ModeGoKey, handler)

	result := protocol.ResultMsg{
		V:         protocol.ProtocolVersion,
		Type:      protocol.TypeResult,
		Seq:       1001,
		HasAction: true,
		Action: protocol.Action{
			Command:        protocol.CmdKick,
			TargetMemberID: "mem_789",
			GroupID:        "group_42",
			ReplyText:      "Removed for spam.",
			BlockHash:      "sha256:abc123",
		},
		Signature: "test_sig",
	}

	err := p.HandleResult(context.Background(), result)
	if err != nil {
		t.Fatalf("HandleResult() error: %v", err)
	}

	if receivedCmd != protocol.CmdKick {
		t.Errorf("handler received command %q, want %q", receivedCmd, protocol.CmdKick)
	}

	stats := p.Stats()
	if stats.ResultsReceived != 1 {
		t.Errorf("ResultsReceived = %d, want 1", stats.ResultsReceived)
	}
	if stats.CommandsExec != 1 {
		t.Errorf("CommandsExec = %d, want 1", stats.CommandsExec)
	}
}

func TestBlockHash(t *testing.T) {
	data := []byte("test block data")
	hash := BlockHash(data)

	if len(hash) < 10 {
		t.Errorf("BlockHash() too short: %q", hash)
	}
	if hash[:7] != "sha256:" {
		t.Errorf("BlockHash() should start with 'sha256:', got %q", hash[:7])
	}

	// Same input must produce same hash.
	hash2 := BlockHash(data)
	if hash != hash2 {
		t.Error("BlockHash() not deterministic")
	}

	// Different input must produce different hash.
	hash3 := BlockHash([]byte("different data"))
	if hash == hash3 {
		t.Error("BlockHash() should differ for different inputs")
	}
}

func TestHandleResultNoHandler(t *testing.T) {
	// Proxy with nil handler should not panic.
	p := New(testLogger(), ModeStandalone, nil)

	result := protocol.ResultMsg{
		V:         protocol.ProtocolVersion,
		Type:      protocol.TypeResult,
		HasAction: true,
		Action: protocol.Action{
			Command: protocol.CmdWarn,
		},
	}

	err := p.HandleResult(context.Background(), result)
	if err != nil {
		t.Fatalf("HandleResult() with nil handler error: %v", err)
	}
}
