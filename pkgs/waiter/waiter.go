package waiter

import "sync"

type Waiter struct {
	mu   sync.Mutex
	wait map[string]chan struct{}
}

func NewWaiter() *Waiter {
	return &Waiter{wait: make(map[string]chan struct{})}
}

func (w *Waiter) Create(id string) func() {
	w.mu.Lock()
	defer w.mu.Unlock()
	ch := make(chan struct{})
	w.wait[id] = ch
	return func() {
		<-ch
	}
}

func (w *Waiter) Ack(id string) bool {
	w.mu.Lock()
	ch, ok := w.wait[id]
	if ok {
		delete(w.wait, id)
	}
	w.mu.Unlock()

	if !ok {
		return false
	}
	close(ch) // 唤醒等待者
	return true
}
