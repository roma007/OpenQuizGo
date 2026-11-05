package battle

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type eventBroker struct {
	rdb    *redis.Client
	mu     sync.Mutex
	subs   map[string]map[chan int64]struct{} // 进程内回退总线
	shared map[string]*sharedSubscription     // 同一实例内共享的 Redis 订阅
	// localVers 保存每个对局的进程内版本计数：Redis 不可用时兜底，
	// 同时记录从 Redis 观察到的最新版本以避免回退后版本倒退。
	localVers map[string]*atomic.Int64
}

// sharedSubscription 表示同一实例内对某个对局共享的一个 Redis PubSub。
// refs 与 subs 由 eventBroker.mu 保护。
type sharedSubscription struct {
	pubsub *redis.PubSub
	subs   map[chan int64]struct{}
	refs   int
}

func newEventBroker(rdb *redis.Client) *eventBroker {
	return &eventBroker{
		rdb:       rdb,
		subs:      make(map[string]map[chan int64]struct{}),
		shared:    make(map[string]*sharedSubscription),
		localVers: make(map[string]*atomic.Int64),
	}
}

func battleChannel(battleID string) string {
	return "battle:events:" + battleID
}

func battleVersionKey(battleID string) string {
	return "battle:events:ver:" + battleID
}

func (b *eventBroker) subscribe(ctx context.Context, battleID string) (<-chan int64, func()) {
	ch := make(chan int64, 1)
	if b.rdb != nil && b.subscribeShared(ctx, battleID, ch) {
		return ch, func() { b.unsubscribeShared(battleID, ch) }
	}
	return b.subscribeLocal(battleID, ch)
}

// subscribeShared 尝试把订阅者挂到共享的 Redis PubSub 上，必要时创建它。
// 返回 false 表示 Redis 订阅失败，调用方应回退到进程内总线。
func (b *eventBroker) subscribeShared(ctx context.Context, battleID string, ch chan int64) bool {
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
		subs:   map[chan int64]struct{}{ch: {}},
		refs:   1,
	}
	b.mu.Unlock()

	go b.forward(battleID, pubsub)
	return true
}

// forward 把共享 PubSub 收到的事件广播给本地所有订阅者，并保留消息中的版本号。
func (b *eventBroker) forward(battleID string, pubsub *redis.PubSub) {
	for msg := range pubsub.Channel() {
		version, err := strconv.ParseInt(msg.Payload, 10, 64)
		if err != nil {
			continue
		}
		b.observeVersion(battleID, version)
		b.notify(battleID, version, true, false)
	}
}

func (b *eventBroker) unsubscribeShared(battleID string, ch chan int64) {
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

func (b *eventBroker) subscribeLocal(battleID string, ch chan int64) (<-chan int64, func()) {
	b.mu.Lock()
	if b.subs[battleID] == nil {
		b.subs[battleID] = make(map[chan int64]struct{})
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

// counter 返回对局的进程内版本计数；map 由 b.mu 保护，原子计数本身无锁。
func (b *eventBroker) counter(battleID string) *atomic.Int64 {
	b.mu.Lock()
	counter := b.localVers[battleID]
	if counter == nil {
		counter = &atomic.Int64{}
		b.localVers[battleID] = counter
	}
	b.mu.Unlock()
	return counter
}

// observeVersion 记录从 Redis 观察到的最新版本，保证后续本地兜底不会倒退。
func (b *eventBroker) observeVersion(battleID string, version int64) {
	counter := b.counter(battleID)
	for {
		current := counter.Load()
		if version <= current {
			return
		}
		if counter.CompareAndSwap(current, version) {
			return
		}
	}
}

// nextVersion 优先使用 Redis INCR 保证跨实例单调递增；
// Redis 未配置或失败时回退到进程内原子计数。
func (b *eventBroker) nextVersion(ctx context.Context, battleID string) int64 {
	counter := b.counter(battleID)
	if b.rdb != nil {
		if version, err := b.rdb.Incr(ctx, battleVersionKey(battleID)).Result(); err == nil {
			b.observeVersion(battleID, version)
			return version
		}
	}
	for {
		current := counter.Load()
		next := current + 1
		if counter.CompareAndSwap(current, next) {
			return next
		}
	}
}

// currentVersion 返回对局当前已知版本基线，不产生新的版本号。
func (b *eventBroker) currentVersion(ctx context.Context, battleID string) int64 {
	counter := b.counter(battleID)
	if b.rdb != nil {
		// 基线只是可选的初始提示，不能阻塞事件流建立。
		readCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		version, err := b.rdb.Get(readCtx, battleVersionKey(battleID)).Int64()
		cancel()
		if err == nil {
			b.observeVersion(battleID, version)
			return version
		}
	}
	return counter.Load()
}

func (b *eventBroker) publish(ctx context.Context, battleID string) int64 {
	version := b.nextVersion(ctx, battleID)
	if b.rdb != nil {
		if err := b.rdb.Publish(ctx, battleChannel(battleID), strconv.FormatInt(version, 10)).Err(); err == nil {
			// 共享订阅者会由 Redis 推送；仍需通知因订阅失败而落在进程内总线的订阅者。
			b.notify(battleID, version, false, true)
			return version
		}
	}
	// Redis 不可用时，共享订阅者与本地订阅者都由本进程直接通知。
	b.notify(battleID, version, true, true)
	return version
}

// notify 在锁内对订阅者做快照，锁外投递，避免慢订阅者阻塞订阅/取消操作。
func (b *eventBroker) notify(battleID string, version int64, includeShared, includeLocal bool) {
	b.mu.Lock()
	var targets []chan int64
	if includeShared {
		if shared := b.shared[battleID]; shared != nil {
			targets = make([]chan int64, 0, len(shared.subs))
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
		case ch <- version:
		default:
		}
	}
}
