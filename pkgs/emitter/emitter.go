package emitter

import (
	"fmt"
	"sync"
)

// ListenerFunc 事件监听器函数类型
type ListenerFunc func(...interface{})

// listener 内部监听器结构
type listener struct {
	id      int
	fn      ListenerFunc
	once    bool
	removed bool
}

// EventEmitter 事件发射器
type EventEmitter struct {
	mu           sync.RWMutex
	events       map[string][]*listener
	nextID       int
	maxListeners int
}

// New 创建一个新的 EventEmitter 实例
func New() *EventEmitter {
	return &EventEmitter{
		events:       make(map[string][]*listener),
		maxListeners: 10, // 默认最大监听器数量
	}
}

// On 添加一个事件监听器（别名：AddListener）
func (e *EventEmitter) On(event string, fn ListenerFunc) *EventEmitter {
	return e.addListener(event, fn, false)
}

// AddListener 添加一个事件监听器
func (e *EventEmitter) AddListener(event string, fn ListenerFunc) *EventEmitter {
	return e.On(event, fn)
}

// Once 添加一个只执行一次的事件监听器
func (e *EventEmitter) Once(event string, fn ListenerFunc) *EventEmitter {
	return e.addListener(event, fn, true)
}

// Off 移除指定的事件监听器（别名：RemoveListener）
func (e *EventEmitter) Off(event string, listenerID int) *EventEmitter {
	e.mu.Lock()
	defer e.mu.Unlock()

	if listeners, ok := e.events[event]; ok {
		for i, l := range listeners {
			if l.id == listenerID {
				l.removed = true
				// 从切片中移除
				e.events[event] = append(listeners[:i], listeners[i+1:]...)
				break
			}
		}
		// 如果没有监听器了，删除这个事件
		if len(e.events[event]) == 0 {
			delete(e.events, event)
		}
	}

	return e
}

// RemoveListener 移除指定的事件监听器
func (e *EventEmitter) RemoveListener(event string, listenerID int) *EventEmitter {
	return e.Off(event, listenerID)
}

// RemoveAllListeners 移除指定事件的所有监听器，如果不指定事件则移除所有事件的所有监听器
func (e *EventEmitter) RemoveAllListeners(events ...string) *EventEmitter {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(events) == 0 {
		// 移除所有事件
		e.events = make(map[string][]*listener)
	} else {
		// 移除指定事件
		for _, event := range events {
			delete(e.events, event)
		}
	}

	return e
}

// Emit 触发事件，调用所有监听器
func (e *EventEmitter) Emit(event string, args ...interface{}) bool {
	e.mu.RLock()
	listeners := e.events[event]
	if len(listeners) == 0 {
		e.mu.RUnlock()
		return false
	}

	// 复制监听器列表，避免在回调中修改原列表导致问题
	listenersCopy := make([]*listener, len(listeners))
	copy(listenersCopy, listeners)
	e.mu.RUnlock()

	// 收集需要移除的 once 监听器
	onceListeners := make([]int, 0)

	for _, l := range listenersCopy {
		if l.removed {
			continue
		}

		// 执行监听器
		l.fn(args...)

		// 如果是 once 监听器，标记为需要移除
		if l.once {
			onceListeners = append(onceListeners, l.id)
		}
	}

	// 移除 once 监听器
	if len(onceListeners) > 0 {
		e.mu.Lock()
		for _, id := range onceListeners {
			if listeners, ok := e.events[event]; ok {
				for i, l := range listeners {
					if l.id == id {
						e.events[event] = append(listeners[:i], listeners[i+1:]...)
						break
					}
				}
			}
		}
		// 如果没有监听器了，删除这个事件
		if len(e.events[event]) == 0 {
			delete(e.events, event)
		}
		e.mu.Unlock()
	}

	return true
}

// ListenerCount 返回指定事件的监听器数量
func (e *EventEmitter) ListenerCount(event string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return len(e.events[event])
}

// EventNames 返回所有已注册事件的名称
func (e *EventEmitter) EventNames() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	names := make([]string, 0, len(e.events))
	for name := range e.events {
		names = append(names, name)
	}

	return names
}

// SetMaxListeners 设置最大监听器数量（0 表示无限制）
func (e *EventEmitter) SetMaxListeners(n int) *EventEmitter {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.maxListeners = n
	return e
}

// GetMaxListeners 获取最大监听器数量
func (e *EventEmitter) GetMaxListeners() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.maxListeners
}

// addListener 内部方法：添加监听器
func (e *EventEmitter) addListener(event string, fn ListenerFunc, once bool) *EventEmitter {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.nextID++
	l := &listener{
		id:   e.nextID,
		fn:   fn,
		once: once,
	}

	if e.events[event] == nil {
		e.events[event] = make([]*listener, 0)
	}

	e.events[event] = append(e.events[event], l)

	// 检查是否超过最大监听器数量
	if e.maxListeners > 0 && len(e.events[event]) > e.maxListeners {
		fmt.Printf("Warning: possible EventEmitter memory leak detected. %d %s listeners added. Use SetMaxListeners() to increase limit\n",
			len(e.events[event]), event)
	}

	return e
}

// Listeners 返回指定事件的所有监听器 ID
func (e *EventEmitter) Listeners(event string) []int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if listeners, ok := e.events[event]; ok {
		ids := make([]int, 0, len(listeners))
		for _, l := range listeners {
			if !l.removed {
				ids = append(ids, l.id)
			}
		}
		return ids
	}

	return []int{}
}
