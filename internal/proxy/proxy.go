// Package proxy bridges the gap between raw SMP blocks and the
// GoKey Wire Protocol. It converts incoming SMP blocks into
// protocol.BlockMsg frames and routes them to the appropriate
// handler (GoKey via WSS or local standalone processing).
package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/saschadaemgen/gobot/internal/protocol"
	"github.com/saschadaemgen/gobot/internal/smp"
)

// ResultHandler is called when a result (real or dummy) is received
// back from GoKey or from standalone processing. The proxy verifies
// the result and executes the command if valid.
type ResultHandler func(ctx context.Context, result protocol.ResultMsg) error

// Mode indicates whether the proxy operates with GoKey or standalone.
type Mode int

const (
	ModeStandalone Mode = iota
	ModeGoKey
)

// String returns a human-readable mode name.
func (m Mode) String() string {
	switch m {
	case ModeStandalone:
		return "standalone"
	case ModeGoKey:
		return "gokey"
	default:
		return "unknown"
	}
}

// Stats tracks proxy metrics.
type Stats struct {
	BlocksReceived  int64
	BlocksForwarded int64
	BlocksDropped   int64
	ResultsReceived int64
	CommandsExec    int64
	Errors          int64
}

// Proxy receives raw SMP blocks, wraps them as Wire Protocol
// BlockMsg frames, and routes them for processing.
type Proxy struct {
	mu      sync.RWMutex
	log     *slog.Logger
	mode    Mode
	stats   Stats
	handler ResultHandler
}

// New creates a new block forwarding proxy.
func New(log *slog.Logger, mode Mode, handler ResultHandler) *Proxy {
	return &Proxy{
		log:     log,
		mode:    mode,
		handler: handler,
	}
}

// Mode returns the current operating mode.
func (p *Proxy) Mode() Mode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode
}

// Stats returns a snapshot of current proxy metrics.
func (p *Proxy) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// HandleBlock is the smp.BlockHandler implementation. It receives
// raw SMP blocks from the SMP manager, wraps them as Wire Protocol
// messages, and forwards them for processing.
func (p *Proxy) HandleBlock(ctx context.Context, block smp.Block) error {
	p.mu.Lock()
	p.stats.BlocksReceived++
	p.mu.Unlock()

	// Build the Wire Protocol BlockMsg.
	msg, err := p.wrapBlock(block)
	if err != nil {
		p.mu.Lock()
		p.stats.Errors++
		p.mu.Unlock()
		return fmt.Errorf("wrap block: %w", err)
	}

	// Encode to constant-size frame.
	frame, err := protocol.Encode(msg)
	if err != nil {
		p.mu.Lock()
		p.stats.BlocksDropped++
		p.stats.Errors++
		p.mu.Unlock()
		p.log.Error("frame encoding failed",
			"server", block.Server,
			"queue", block.QueueID,
			"error", err)
		return fmt.Errorf("frame encode: %w", err)
	}

	p.mu.Lock()
	p.stats.BlocksForwarded++
	p.mu.Unlock()

	p.log.Debug("block forwarded",
		"server", block.Server,
		"queue", block.QueueID,
		"frame_size", len(frame),
		"mode", p.mode.String())

	// TODO(sprint2): In GoKey mode, send frame via WSS.
	// TODO(sprint3): In standalone mode, decrypt locally.
	_ = frame

	return nil
}

// HandleResult processes a result received from GoKey or from
// standalone local processing.
func (p *Proxy) HandleResult(ctx context.Context, result protocol.ResultMsg) error {
	p.mu.Lock()
	p.stats.ResultsReceived++
	p.mu.Unlock()

	if !result.HasAction {
		// Dummy result - discard silently.
		p.log.Debug("dummy result discarded", "ref_id", result.RefID)
		return nil
	}

	p.log.Info("command received",
		"command", result.Action.Command,
		"target", result.Action.TargetMemberID,
		"group", result.Action.GroupID)

	if p.handler != nil {
		if err := p.handler(ctx, result); err != nil {
			p.mu.Lock()
			p.stats.Errors++
			p.mu.Unlock()
			return fmt.Errorf("result handler: %w", err)
		}
	}

	p.mu.Lock()
	p.stats.CommandsExec++
	p.mu.Unlock()

	return nil
}

// wrapBlock converts a raw SMP block into a Wire Protocol BlockMsg.
func (p *Proxy) wrapBlock(block smp.Block) (protocol.BlockMsg, error) {
	payload := base64.StdEncoding.EncodeToString(block.Data)

	return protocol.BlockMsg{
		V:       protocol.ProtocolVersion,
		Type:    protocol.TypeBlock,
		ID:      generateID(),
		TS:      block.ReceivedAt.UTC().Format(time.RFC3339Nano),
		QueueID: block.Server + "#" + block.QueueID,
		GroupID: "", // TODO(sprint3): resolve from queue mapping
		Sender: protocol.Sender{
			MemberID:    "", // extracted after decryption
			DisplayName: "",
			Role:        "",
			ContactID:   "",
		},
		Payload: payload,
	}, nil
}

// BlockHash computes the SHA-256 hash of raw block data.
// Used for context binding in command signatures.
func BlockHash(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + fmt.Sprintf("%x", h)
}

// generateID produces a unique message ID.
// TODO: Replace with proper UUIDv4 generation.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
