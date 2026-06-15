package global

import (
	"os"
	"path/filepath"
	"sync"
)

const cacheVersion = "18"

type Paths struct {
	Home   string
	Data   string
	Cache  string
	Config string
	State  string
	Log    string
	Bin    string
}

var (
	Path Paths
	once sync.Once
)

func Init() error {
	var initErr error
	once.Do(func() {
		home := os.Getenv(EnvTestHome)
		if home == "" {
			var err error
			home, err = os.UserHomeDir()
			if err != nil {
				initErr = err
				return
			}
		}

		dataHome := envOrDefault("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
		cacheHome := envOrDefault("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
		configHome := envOrDefault("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		stateHome := envOrDefault("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

		Path = Paths{
			Home:   home,
			Data:   filepath.Join(dataHome, AppName),
			Cache:  filepath.Join(cacheHome, AppName),
			Config: filepath.Join(configHome, AppName),
			State:  filepath.Join(stateHome, AppName),
		}
		Path.Log = filepath.Join(Path.Data, "log")
		Path.Bin = filepath.Join(Path.Data, "bin")

		for _, dir := range []string{Path.Data, Path.Cache, Path.Config, Path.State, Path.Log, Path.Bin} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				initErr = err
				return
			}
		}

		if err := bumpCacheVersion(); err != nil {
			initErr = err
			return
		}
	})
	return initErr
}

func envOrDefault(name string, value string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return value
}

func bumpCacheVersion() error {
	versionPath := filepath.Join(Path.Cache, "version")
	contents, err := os.ReadFile(versionPath)
	if err == nil && string(contents) == cacheVersion {
		return nil
	}
	entries, err := os.ReadDir(Path.Cache)
	if err == nil {
		for _, entry := range entries {
			_ = os.RemoveAll(filepath.Join(Path.Cache, entry.Name()))
		}
	}
	return os.WriteFile(versionPath, []byte(cacheVersion), 0o644)
}

func ResetForTest() {
	once = sync.Once{}
	Path = Paths{}
}
