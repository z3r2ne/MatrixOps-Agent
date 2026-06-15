package project

import "sync"

// State provides a per-directory cache tied to the current Instance.
func State[T any](init func(*Instance) (T, error)) func() (T, error) {
	var mu sync.Mutex
	items := map[string]T{}
	return func() (T, error) {
		inst := Current()
		var zero T
		if inst == nil {
			return zero, ErrNoInstance
		}
		mu.Lock()
		defer mu.Unlock()
		if value, ok := items[inst.Directory]; ok {
			return value, nil
		}
		value, err := init(inst)
		if err != nil {
			return zero, err
		}
		items[inst.Directory] = value
		return value, nil
	}
}

var ErrNoInstance = errNoInstance("no active instance")

type errNoInstance string

func (e errNoInstance) Error() string {
	return string(e)
}
