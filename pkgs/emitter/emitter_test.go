package emitter

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	e := New()
	if e == nil {
		t.Fatal("New() returned nil")
	}
	if e.maxListeners != 10 {
		t.Errorf("Expected maxListeners to be 10, got %d", e.maxListeners)
	}
}

func TestOn(t *testing.T) {
	e := New()
	called := false

	e.On("test", func(args ...interface{}) {
		called = true
	})

	e.Emit("test")

	if !called {
		t.Error("Listener was not called")
	}
}

func TestOnce(t *testing.T) {
	e := New()
	count := 0

	e.Once("test", func(args ...interface{}) {
		count++
	})

	e.Emit("test")
	e.Emit("test")
	e.Emit("test")

	if count != 1 {
		t.Errorf("Expected listener to be called once, but was called %d times", count)
	}
}

func TestEmitWithArgs(t *testing.T) {
	e := New()
	var receivedArgs []interface{}

	e.On("test", func(args ...interface{}) {
		receivedArgs = args
	})

	e.Emit("test", "hello", 42, true)

	if len(receivedArgs) != 3 {
		t.Fatalf("Expected 3 arguments, got %d", len(receivedArgs))
	}

	if receivedArgs[0] != "hello" {
		t.Errorf("Expected first arg to be 'hello', got %v", receivedArgs[0])
	}
	if receivedArgs[1] != 42 {
		t.Errorf("Expected second arg to be 42, got %v", receivedArgs[1])
	}
	if receivedArgs[2] != true {
		t.Errorf("Expected third arg to be true, got %v", receivedArgs[2])
	}
}

func TestOff(t *testing.T) {
	e := New()
	count := 0

	e.On("test", func(args ...interface{}) {
		count++
	})

	listeners := e.Listeners("test")
	if len(listeners) != 1 {
		t.Fatalf("Expected 1 listener, got %d", len(listeners))
	}

	e.Off("test", listeners[0])

	e.Emit("test")

	if count != 0 {
		t.Errorf("Expected listener to not be called after removal, but was called %d times", count)
	}
}

func TestRemoveAllListeners(t *testing.T) {
	e := New()
	count1 := 0
	count2 := 0

	e.On("test1", func(args ...interface{}) {
		count1++
	})

	e.On("test2", func(args ...interface{}) {
		count2++
	})

	e.RemoveAllListeners("test1")
	e.Emit("test1")
	e.Emit("test2")

	if count1 != 0 {
		t.Errorf("Expected test1 listener to not be called, but was called %d times", count1)
	}
	if count2 != 1 {
		t.Errorf("Expected test2 listener to be called once, but was called %d times", count2)
	}
}

func TestRemoveAllListenersNoArgs(t *testing.T) {
	e := New()
	count1 := 0
	count2 := 0

	e.On("test1", func(args ...interface{}) {
		count1++
	})

	e.On("test2", func(args ...interface{}) {
		count2++
	})

	e.RemoveAllListeners()
	e.Emit("test1")
	e.Emit("test2")

	if count1 != 0 {
		t.Errorf("Expected test1 listener to not be called, but was called %d times", count1)
	}
	if count2 != 0 {
		t.Errorf("Expected test2 listener to not be called, but was called %d times", count2)
	}
}

func TestListenerCount(t *testing.T) {
	e := New()

	e.On("test", func(args ...interface{}) {})
	e.On("test", func(args ...interface{}) {})
	e.On("test", func(args ...interface{}) {})

	count := e.ListenerCount("test")
	if count != 3 {
		t.Errorf("Expected 3 listeners, got %d", count)
	}
}

func TestEventNames(t *testing.T) {
	e := New()

	e.On("event1", func(args ...interface{}) {})
	e.On("event2", func(args ...interface{}) {})
	e.On("event3", func(args ...interface{}) {})

	names := e.EventNames()
	if len(names) != 3 {
		t.Errorf("Expected 3 event names, got %d", len(names))
	}

	// 检查是否包含所有事件名
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	if !nameMap["event1"] || !nameMap["event2"] || !nameMap["event3"] {
		t.Error("Not all event names are present")
	}
}

func TestSetMaxListeners(t *testing.T) {
	e := New()
	e.SetMaxListeners(5)

	max := e.GetMaxListeners()
	if max != 5 {
		t.Errorf("Expected maxListeners to be 5, got %d", max)
	}
}

func TestMultipleListeners(t *testing.T) {
	e := New()
	count1 := 0
	count2 := 0
	count3 := 0

	e.On("test", func(args ...interface{}) {
		count1++
	})

	e.On("test", func(args ...interface{}) {
		count2++
	})

	e.On("test", func(args ...interface{}) {
		count3++
	})

	e.Emit("test")

	if count1 != 1 || count2 != 1 || count3 != 1 {
		t.Errorf("Expected all listeners to be called once, got counts: %d, %d, %d", count1, count2, count3)
	}
}

func TestConcurrentEmit(t *testing.T) {
	e := New()
	var mu sync.Mutex
	count := 0

	e.On("test", func(args ...interface{}) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.Emit("test")
		}()
	}

	wg.Wait()

	if count != 100 {
		t.Errorf("Expected listener to be called 100 times, but was called %d times", count)
	}
}

func TestConcurrentOnOff(t *testing.T) {
	e := New()
	var wg sync.WaitGroup

	// 并发添加和移除监听器
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.On("test", func(args ...interface{}) {})
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			listeners := e.Listeners("test")
			if len(listeners) > 0 {
				e.Off("test", listeners[0])
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			e.Emit("test")
		}()
	}

	wg.Wait()

	// 测试通过表示没有死锁或 panic
}

func TestEmitReturnValue(t *testing.T) {
	e := New()

	// 没有监听器时应返回 false
	result := e.Emit("nonexistent")
	if result {
		t.Error("Expected Emit to return false when no listeners exist")
	}

	e.On("test", func(args ...interface{}) {})

	// 有监听器时应返回 true
	result = e.Emit("test")
	if !result {
		t.Error("Expected Emit to return true when listeners exist")
	}
}

func TestChaining(t *testing.T) {
	e := New()
	count := 0

	// 测试方法链
	e.On("test1", func(args ...interface{}) {
		count++
	}).On("test2", func(args ...interface{}) {
		count++
	}).SetMaxListeners(20)

	e.Emit("test1")
	e.Emit("test2")

	if count != 2 {
		t.Errorf("Expected count to be 2, got %d", count)
	}

	if e.GetMaxListeners() != 20 {
		t.Errorf("Expected maxListeners to be 20, got %d", e.GetMaxListeners())
	}
}

func TestOnceWithMultipleEmits(t *testing.T) {
	e := New()
	results := []string{}
	var mu sync.Mutex

	e.Once("test", func(args ...interface{}) {
		mu.Lock()
		results = append(results, args[0].(string))
		mu.Unlock()
	})

	e.Emit("test", "first")
	time.Sleep(10 * time.Millisecond)
	e.Emit("test", "second")
	time.Sleep(10 * time.Millisecond)
	e.Emit("test", "third")

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d: %v", len(results), results)
	}

	if results[0] != "first" {
		t.Errorf("Expected 'first', got '%s'", results[0])
	}
}
