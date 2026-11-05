package battle

import "sync"

type eventBroker struct {
	mu   sync.Mutex
	subs map[string]map[chan struct{}]struct{}
}

func newEventBroker() *eventBroker {
	return &eventBroker{subs: make(map[string]map[chan struct{}]struct{})}
}

func (b *eventBroker) subscribe(battleID string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
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

func (b *eventBroker) publish(battleID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[battleID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
