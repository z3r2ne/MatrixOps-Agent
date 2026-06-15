package bus

import "sync"

const (
	AllEventName = "_all_"
)

type Event struct {
	Name    string
	Payload interface{}
}

type Subscriber func(Event)

// Bus 是一个事件总线对象，管理事件的发布和订阅
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string]map[int]Subscriber
	nextID      int
}

// New 创建一个新的 Bus 实例
func New() *Bus {
	return &Bus{
		subscribers: make(map[string]map[int]Subscriber),
	}
}

// Publish 发布一个事件到所有订阅者
func (b *Bus) Publish(name string, payload interface{}) {
	b.mu.RLock()
	list := make([]Subscriber, 0, len(b.subscribers[name])+len(b.subscribers[AllEventName]))
	for _, handler := range b.subscribers[name] {
		list = append(list, handler)
	}
	for _, handler := range b.subscribers[AllEventName] {
		list = append(list, handler)
	}
	b.mu.RUnlock()
	if len(list) == 0 {
		return
	}
	event := Event{Name: name, Payload: payload}
	for _, handler := range list {
		handler(event)
	}
}

func (b *Bus) SubscribeAll(handler Subscriber) func() {
	return b.Subscribe(AllEventName, handler)
}

// Subscribe 订阅一个事件，返回一个取消订阅的函数
func (b *Bus) Subscribe(name string, handler Subscriber) func() {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	if b.subscribers[name] == nil {
		b.subscribers[name] = map[int]Subscriber{}
	}
	b.subscribers[name][id] = handler
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if list, ok := b.subscribers[name]; ok {
			delete(list, id)
			if len(list) == 0 {
				delete(b.subscribers, name)
			}
		}
	}
}

// 全局默认实例，用于向后兼容
var defaultBus = New()

// Publish 使用默认 Bus 实例发布事件
func Publish(name string, payload interface{}) {
	defaultBus.Publish(name, payload)
}

// Subscribe 使用默认 Bus 实例订阅事件
func Subscribe(name string, handler Subscriber) func() {
	return defaultBus.Subscribe(name, handler)
}
