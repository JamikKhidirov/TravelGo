package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Event struct {
	Key   string
	Value []byte
	Time  time.Time
}

// MemoryBroker is an in-memory event broker used when Kafka is not available.
// It implements the same Publish/Subscribe contract as the Kafka broker so the
// rest of the application does not need to change.
type MemoryBroker struct {
	mu          sync.Mutex
	subscribers []chan Event
	closed      bool
}

func NewMemoryBroker() *MemoryBroker {
	return &MemoryBroker{
		subscribers: make([]chan Event, 0),
	}
}

func (b *MemoryBroker) Publish(ctx context.Context, key string, value interface{}) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("broker closed")
	}

	bBytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	event := Event{Key: key, Value: bBytes, Time: time.Now()}

	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		case <-ctx.Done():
			return ctx.Err()
		default:
			// slow subscriber, skip
		}
	}
	return nil
}

func (b *MemoryBroker) Subscribe(ctx context.Context, handler func(Event)) error {
	b.mu.Lock()
	ch := make(chan Event, 100)
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		for i, c := range b.subscribers {
			if c == ch {
				b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
				break
			}
		}
		b.mu.Unlock()
		close(ch)
	}()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			handler(event)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (b *MemoryBroker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for _, ch := range b.subscribers {
		close(ch)
	}
	b.subscribers = nil
	return nil
}

var _ interface {
	Publish(context.Context, string, interface{}) error
	Close() error
} = (*MemoryBroker)(nil)
