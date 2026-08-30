package clipboard

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcherRetriesReadWithoutConsumingSequence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var attempts atomic.Int32
	got := make(chan RawContent, 1)
	w := Watcher{
		Interval: 10 * time.Millisecond,
		Sequence: func() (uint32, bool) { return 7, true },
		ReadSnapshot: func() (RawContent, error) {
			if attempts.Add(1) < 3 {
				return RawContent{}, errors.New("clipboard busy")
			}
			return RawContent{Text: "copied"}, nil
		},
	}
	go w.Start(ctx, func(raw RawContent) { got <- raw })
	select {
	case raw := <-got:
		if raw.Text != "copied" {
			t.Fatalf("raw=%+v", raw)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out after %d attempts", attempts.Load())
	}
	if attempts.Load() < 3 {
		t.Fatalf("attempts=%d, want retries", attempts.Load())
	}
}

func TestWatcherUsesChangeNotificationsAndCoalesces(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes := make(chan struct{}, 1)
	got := make(chan RawContent, 2)
	var reads atomic.Int32
	w := Watcher{Changes: changes, Interval: time.Hour,
		ReadSnapshot: func() (RawContent, error) { reads.Add(1); return RawContent{Text: "x"}, nil }}
	go w.Start(ctx, func(raw RawContent) { got <- raw })
	// Initial capture is expected.
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("initial capture missing")
	}
	// The buffered event must not block even when several clipboard changes
	// arrive before the worker is scheduled. Send them after the initial read so
	// this assertion covers a distinct notification cycle.
	for i := 0; i < 10; i++ {
		select {
		case changes <- struct{}{}:
		default:
		}
	}
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("notification capture missing")
	}
	if reads.Load() > 2 {
		t.Fatalf("reads=%d, expected coalescing", reads.Load())
	}
}
