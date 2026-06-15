package coreagent

import (
	"sync"
)

// MaxConcurrentToolCalls 单轮模型输出中同时执行的工具调用上限。
const MaxConcurrentToolCalls = 10

// runIndexedParallel 使用 WaitGroup 与信号量并发执行 count 个任务，并返回首个错误。
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
