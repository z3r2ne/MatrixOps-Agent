package task_runner

import (
	"context"
	"sync"
)

// mergeContexts returns a derived context that is canceled when any parent context is canceled.
// It preserves values/deadlines from the first non-nil context.
func mergeContexts(contexts ...context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	for _, ctx := range contexts {
		if ctx != nil {
			base = ctx
			break
		}
	}

	mergedCtx, mergedCancelCause := context.WithCancelCause(base)

	var once sync.Once
	cancelWithCause := func(err error) {
		once.Do(func() {
			mergedCancelCause(err)
		})
	}

	for _, parent := range contexts {
		if parent == nil || parent == base {
			continue
		}

		go func(parent context.Context) {
			select {
			case <-parent.Done():
				cancelWithCause(context.Cause(parent))
			case <-mergedCtx.Done():
			}
		}(parent)
	}

	return mergedCtx, func() {
		cancelWithCause(context.Canceled)
	}
}
