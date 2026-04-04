package smp

import (
	"context"
	"testing"
	"time"
)

func TestBackoffSequence(t *testing.T) {
	b := NewBackoff(1*time.Second, 30*time.Second)

	// Expected base delays: 1s, 2s, 4s, 8s, 16s, 30s, 30s
	expected := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
		30 * time.Second,
	}

	for i, want := range expected {
		got := b.Next()
		// Allow 25% jitter tolerance.
		low := time.Duration(float64(want) * 0.74)
		high := time.Duration(float64(want) * 1.26)

		if got < low || got > high {
			t.Errorf("attempt %d: got %v, want %v +/- 25%% (range %v-%v)",
				i, got, want, low, high)
		}
	}
}

func TestBackoffReset(t *testing.T) {
	b := NewBackoff(1*time.Second, 30*time.Second)

	// Advance a few attempts.
	b.Next()
	b.Next()
	b.Next()

	if b.Attempt() != 3 {
		t.Errorf("Attempt() = %d, want 3", b.Attempt())
	}

	b.Reset()

	if b.Attempt() != 0 {
		t.Errorf("Attempt() after Reset() = %d, want 0", b.Attempt())
	}

	// Next delay should be back to base (~1s).
	got := b.Next()
	low := 750 * time.Millisecond
	high := 1250 * time.Millisecond
	if got < low || got > high {
		t.Errorf("after Reset(), Next() = %v, want ~1s (range %v-%v)",
			got, low, high)
	}
}

func TestBackoffCapsAtMax(t *testing.T) {
	b := NewBackoff(1*time.Second, 30*time.Second)

	// Run many attempts - should never exceed max + jitter.
	for i := 0; i < 20; i++ {
		got := b.Next()
		ceiling := time.Duration(float64(30*time.Second) * 1.26)
		if got > ceiling {
			t.Errorf("attempt %d: %v exceeds max with jitter (%v)",
				i, got, ceiling)
		}
	}
}

func TestBackoffDefault(t *testing.T) {
	b := DefaultBackoff()

	got := b.Next()
	low := 750 * time.Millisecond
	high := 1250 * time.Millisecond
	if got < low || got > high {
		t.Errorf("DefaultBackoff() first Next() = %v, want ~1s", got)
	}
}

func TestBackoffWaitCancelled(t *testing.T) {
	b := NewBackoff(10*time.Second, 30*time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately.
	cancel()

	start := time.Now()
	err := b.Wait(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Wait() should return error when context cancelled")
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("Wait() took %v, should return immediately on cancel", elapsed)
	}
}

func TestBackoffWaitCompletes(t *testing.T) {
	// Use tiny delays for fast test.
	b := NewBackoff(10*time.Millisecond, 100*time.Millisecond)

	ctx := context.Background()

	start := time.Now()
	err := b.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Wait() error: %v", err)
	}
	if elapsed < 5*time.Millisecond {
		t.Errorf("Wait() returned too quickly: %v", elapsed)
	}
}

func TestBackoffJitterDistribution(t *testing.T) {
	// Run Next() many times at the same attempt level to verify
	// jitter produces different values.
	seen := make(map[time.Duration]bool)

	for i := 0; i < 50; i++ {
		b := NewBackoff(1*time.Second, 30*time.Second)
		d := b.Next()
		seen[d.Truncate(time.Millisecond)] = true
	}

	// With 25% jitter on 1s, we expect a range of ~750ms-1250ms.
	// 50 samples should produce at least 10 distinct values.
	if len(seen) < 10 {
		t.Errorf("jitter produced only %d distinct values in 50 samples, expected more", len(seen))
	}
}
