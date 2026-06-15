package memorysearch

import "sync"

var (
	defaultStore *Store
	initOnce     sync.Once
	initErr      error
)

// InitStore opens chromem-go and bleve indexes under the app data directory.
func InitStore() error {
	initOnce.Do(func() {
		defaultStore, initErr = openStore()
	})
	return initErr
}

func ensureStore() (*Store, error) {
	if defaultStore == nil {
		if err := InitStore(); err != nil {
			return nil, err
		}
	}
	return defaultStore, nil
}
