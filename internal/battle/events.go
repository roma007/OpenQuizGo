package battle

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
)

type eventBroker struct {
	rdb    *redis.Client
	mu     sync.Mutex
	subs   map[string]map[chan struct{}]struct{} // 进程内回退总线
	shared map[string]*sharedSubscription        // 同一实例内共享的 Redis 订阅
}

// sharedSubscription 表示同一实例内对某个对局共享的一个 Redis PubSub。
// refs 与 subs 由 eventBroker.mu 保护。
type sharedSubscription struct {
	pubsub *redis.PubSub
	subs   map[chan struct{}]struct{}
	refs   int
}

func newEventBroker(rdb *redis.Client) *eventBroker {
	return &eventBroker{
		rdb:    rdb,
		subs:   make(map[string]map[chan struct{}]struct{}),
		shared: make(map[string]*sharedSubscription),
	}
}

func battleChannel(battleID string) string {
	return "battle:events:" + battleID
}

func (b *eventBroker) subscribe(ctx context.Context, battleID string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	if b.rdb != nil && b.subscribeShared(ctx, battleID, ch) {
		return ch, func() { b.unsubscribeShared(battleID, ch) }
	}
	return b.subscribeLocal(battleID, ch)
}

// subscribeShared 尝试把订阅者挂到共享的 Redis PubSub 上，必要时创建它。
// 返回 false 表示 Redis 订阅失败，调用方应回退到进程内总线。
func (b *eventBroker) subscribeShared(ctx context.Context, battleID string, ch chan struct{}) bool {
	b.mu.Lock()
	if shared := b.shared[battleID]; shared != nil {
		shared.subs[ch] = struct{}{}
		shared.refs++
		b.mu.Unlock()
		return true
	}
	// 创建 PubSub 涉及网络往返，不能持锁等待，否则会阻塞其他对局的订阅。
	b.mu.Unlock()

	pubsub := b.rdb.Subscribe(ctx, battleChannel(battleID))
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return false
	}

	b.mu.Lock()
	if shared := b.shared[battleID]; shared != nil {
		// 并发创建时其他订阅者已先建立共享订阅，复用并丢弃本次连接。
		shared.subs[ch] = struct{}{}
		shared.refs++
		b.mu.Unlock()
		_ = pubsub.Close()
		return true
	}
	b.shared[battleID] = &sharedSubscription{
		pubsub: pubsub,
		subs:   map[chan struct{}]struct{}{ch: {}},
		refs:   1,
	}
	b.mu.Unlock()

	go b.forward(battleID, pubsub)
	return true
}

// forward 把共享 PubSub 收到的事件广播给本地所有订阅者。
func (b *eventBroker) forward(battleID string, pubsub *redis.PubSub) {
	for range pubsub.Channel() {
		b.notify(battleID, true, false)
	}
}

func (b *eventBroker) unsubscribeShared(battleID string, ch chan struct{}) {
	b.mu.Lock()
	shared := b.shared[battleID]
	if shared == nil {
		b.mu.Unlock()
		return
	}
	if _, ok := shared.subs[ch]; !ok {
		b.mu.Unlock()
		return
	}
	delete(shared.subs, ch)
	shared.refs--
	if shared.refs > 0 {
		b.mu.Unlock()
		return
	}
	delete(b.shared, battleID)
	b.mu.Unlock()

	// 最后一个订阅者离开后才真正关闭 Redis 订阅。
	_ = shared.pubsub.Close()
}

func (b *eventBroker) subscribeLocal(battleID string, ch chan struct{}) (<-chan struct{}, func()) {
	b.mu.Lock()
	if b.subs[battleID] == nil {
		b.subs[battleID] = make(map[chan struct{}]struct{})
	}
	b.subs[battleID][ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if _, ok := b.subs[battleID][ch]; ok {
			delete(b.subs[battleID], ch)
			if len(b.subs[battleID]) == 0 {
				delete(b.subs, battleID)
			}
		}
		b.mu.Unlock()
	}
}

func (b *eventBroker) publish(ctx context.Context, battleID string) {
	if b.rdb != nil {
		if err := b.rdb.Publish(ctx, battleChannel(battleID), "1").Err(); err == nil {
			// 共享订阅者会由 Redis 推送；仍需通知因订阅失败而落在进程内总线的订阅者。
			b.notify(battleID, false, true)
			return
		}
	}
	b.notify(battleID, true, true)
}

// notify 在锁内对订阅者做快照，锁外投递，避免慢订阅者阻塞订阅/取消操作。
func (b *eventBroker) notify(battleID string, includeShared, includeLocal bool) {
	b.mu.Lock()
	var targets []chan struct{}
	if includeShared {
		if shared := b.shared[battleID]; shared != nil {
			targets = make([]chan struct{}, 0, len(shared.subs))
			for ch := range shared.subs {
				targets = append(targets, ch)
			}
		}
	}
	if includeLocal {
		for ch := range b.subs[battleID] {
			targets = append(targets, ch)
		}
	}
	b.mu.Unlock()

	for _, ch := range targets {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
