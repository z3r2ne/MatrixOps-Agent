package streamtypes

import (
	"sync"
)

// MaxConcurrentToolCalls 与 core_agent 保持一致，供流式工具调度使用。
const MaxConcurrentToolCalls = 10

func runIndexedParallel(count int, maxConcurrency int, fn func(index int) error) error {
	if count <= 0 {
		return nil
	}
	if maxConcurrency <= 0 {
		maxConcurrency = MaxConcurrentToolCalls
	}
	if maxConcurrency > count {
		maxConcurrency = count
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := fn(index); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	return firstErr
}
