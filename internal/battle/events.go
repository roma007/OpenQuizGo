package battle

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
)

type eventBroker struct {
	rdb  *redis.Client
	mu   sync.Mutex
	subs map[string]map[chan struct{}]struct{}
}

func newEventBroker(rdb *redis.Client) *eventBroker {
	return &eventBroker{
		rdb:  rdb,
		subs: make(map[string]map[chan struct{}]struct{}),
	}
}

func battleChannel(battleID string) string {
	return "battle:events:" + battleID
}

func (b *eventBroker) subscribe(ctx context.Context, battleID string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	if b.rdb != nil {
		pubsub := b.rdb.Subscribe(ctx, battleChannel(battleID))
		if _, err := pubsub.Receive(ctx); err == nil {
			go func() {
				defer func() { _ = pubsub.Close() }()
				for range pubsub.Channel() {
					select {
					case ch <- struct{}{}:
					default:
					}
				}
			}()
			return ch, func() { _ = pubsub.Close() }
		}
		_ = pubsub.Close()
	}

	b.mu.Lock()
	if b.subs[battleID] == nil {
		b.subs[battleID] = make(map[chan struct{}]struct{})
	}
	b.subs[battleID][ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs[battleID], ch)
		if len(b.subs[battleID]) == 0 {
			delete(b.subs, battleID)
		}
		b.mu.Unlock()
	}
}

func (b *eventBroker) publish(ctx context.Context, battleID string) {
	if b.rdb != nil {
		if err := b.rdb.Publish(ctx, battleChannel(battleID), "1").Err(); err == nil {
			return
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[battleID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
