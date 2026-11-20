package sdk

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

// A network deadline can wake the read before the context timer publishes Err.
type pendingDeadlineContext struct {
	context.Context
	deadline time.Time
}

func (ctx pendingDeadlineContext) Deadline() (time.Time, bool) { return ctx.deadline, true }

func TestSQLWebSocketDeadlineBeforeContextTimer(t *testing.T) {
	reader, writer := net.Pipe()
	defer reader.Close()
	defer writer.Close()
	ctx := pendingDeadlineContext{context.Background(), time.Now().Add(-time.Millisecond)}
	var received [1]byte
	err := webSocketOperation(ctx, reader.SetReadDeadline, func() error {
		_, err := reader.Read(received[:])
		return err
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired SDK read deadline: %v", err)
	}
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("original network timeout lost: %v", err)
	}
	sent := make(chan error, 1)
	go func() { _, err := writer.Write([]byte{42}); sent <- err }()
	next, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := webSocketOperation(next, reader.SetReadDeadline, func() error {
		_, err := reader.Read(received[:])
		return err
	}); err != nil {
		t.Fatalf("read after deadline reset: %v", err)
	}
	if received[0] != 42 {
		t.Fatalf("received %d", received[0])
	}
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
}

func TestSQLWebSocketPreservesOtherErrors(t *testing.T) {
	timeout := &net.OpError{Op: "read", Net: "pipe", Err: os.ErrDeadlineExceeded}
	other := errors.New("connection closed")
	for _, test := range []struct {
		name     string
		deadline time.Time
		cause    error
	}{
		{"timeout before SDK deadline", time.Now().Add(time.Hour), timeout},
		{"non-timeout after deadline", time.Now().Add(-time.Hour), other},
		{"timeout without SDK deadline", time.Time{}, timeout},
	} {
		t.Run(test.name, func(t *testing.T) {
			var ctx context.Context = t.Context()
			if !test.deadline.IsZero() {
				ctx = pendingDeadlineContext{ctx, test.deadline}
			}
			err := webSocketOperation(ctx, func(time.Time) error { return nil }, func() error { return test.cause })
			if !errors.Is(err, test.cause) || errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("unexpected reclassification: %v", err)
			}
		})
	}
}
