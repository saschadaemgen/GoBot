package smp

import (
	"context"
	"math"
	"math/rand/v2"
	"time"

	"github.com/saschadaemgen/gobot/internal/protocol"
)

// Backoff implements exponential backoff with jitter for
// reconnection attempts. Used for both SMP server connections
// and GoKey WSS reconnection.
//
// Backoff sequence: 1s, 2s, 4s, 8s, 16s, 30s, 30s, 30s...
// Jitter adds +/- 25% randomization to prevent thundering herd.
type Backoff struct {
	base    time.Duration
	max     time.Duration
	attempt int
}

// NewBackoff creates a backoff timer with the given base delay
// and maximum delay. Matches wire protocol constants:
// base = 1s, max = 30s.
func NewBackoff(base, max time.Duration) *Backoff {
	return &Backoff{
		base: base,
		max:  max,
	}
}

// DefaultBackoff creates a backoff with wire protocol defaults
// (1s base, 30s max).
func DefaultBackoff() *Backoff {
	return NewBackoff(
		time.Duration(protocol.ReconnectBase)*time.Second,
		time.Duration(protocol.ReconnectMax)*time.Second,
	)
}

// Next returns the next backoff duration and increments the
// attempt counter. The duration includes jitter.
func (b *Backoff) Next() time.Duration {
	delay := b.delay()
	b.attempt++
	return addJitter(delay)
}

// Reset sets the attempt counter back to zero. Call after a
// successful connection.
func (b *Backoff) Reset() {
	b.attempt = 0
}

// Attempt returns the current attempt number (0-based).
func (b *Backoff) Attempt() int {
	return b.attempt
}

// Wait blocks for the next backoff duration or until the context
// is cancelled. Returns the context error if cancelled, nil if
// the wait completed.
func (b *Backoff) Wait(ctx context.Context) error {
	delay := b.Next()
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// delay calculates the raw delay without jitter.
func (b *Backoff) delay() time.Duration {
	if b.attempt == 0 {
		return b.base
	}

	// 2^attempt * base, capped at max.
	factor := math.Pow(2, float64(b.attempt))
	delay := time.Duration(float64(b.base) * factor)

	if delay > b.max {
		return b.max
	}
	return delay
}

// addJitter adds +/- 25% randomization to prevent multiple
// clients from reconnecting at exactly the same time.
func addJitter(d time.Duration) time.Duration {
	jitter := float64(d) * 0.25
	offset := (rand.Float64() * 2 * jitter) - jitter
	result := time.Duration(float64(d) + offset)
	if result < 0 {
		return 0
	}
	return result
}
