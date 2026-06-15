package provider

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"matrixops-agent/global"
)

type ModelsDevModel struct {
	ID    string `json:"id"`
	Limit struct {
		Context int `json:"context"`
		Input   int `json:"input"`
		Output  int `json:"output"`
	} `json:"limit"`
	Cost     *ModelsDevCost                    `json:"cost"`
	Options  map[string]interface{}            `json:"options"`
	Headers  map[string]string                 `json:"headers"`
	Variants map[string]map[string]interface{} `json:"variants"`
	Provider *struct {
		NPM string `json:"npm"`
	} `json:"provider"`
}

type ModelsDevCost struct {
	Input           float64 `json:"input"`
	Output          float64 `json:"output"`
	CacheRead       float64 `json:"cache_read"`
	CacheWrite      float64 `json:"cache_write"`
	ContextOver200K *struct {
		Input      float64 `json:"input"`
		Output     float64 `json:"output"`
		CacheRead  float64 `json:"cache_read"`
		CacheWrite float64 `json:"cache_write"`
	} `json:"context_over_200k"`
}

type ModelsDevProvider struct {
	API    string                    `json:"api"`
	ID     string                    `json:"id"`
	NPM    string                    `json:"npm"`
	Models map[string]ModelsDevModel `json:"models"`
}

var (
	modelsDevMu          sync.Mutex
	modelsDevCache       map[string]ModelsDevProvider
	modelsDevLoaded      bool
	modelsDevRefreshOnce sync.Once
)

func LoadModelsDev() (map[string]ModelsDevProvider, error) {
	modelsDevMu.Lock()
	defer modelsDevMu.Unlock()
	if modelsDevLoaded {
		return modelsDevCache, nil
	}
	modelsDevLoaded = true

	if err := global.Init(); err != nil {
		return nil, err
	}
	cachePath := filepath.Join(global.Path.Cache, "models.json")
	if data, err := os.ReadFile(cachePath); err == nil {
		var parsed map[string]ModelsDevProvider
		if json.Unmarshal(data, &parsed) == nil {
			modelsDevCache = parsed
			return modelsDevCache, nil
		}
	}
	if os.Getenv(global.EnvDisableModelsFetch) != "" {
		modelsDevCache = map[string]ModelsDevProvider{}
		return modelsDevCache, nil
	}
	fetched, err := fetchModelsDev(cachePath)
	if err != nil {
		modelsDevCache = map[string]ModelsDevProvider{}
		startModelsDevRefresh()
		return modelsDevCache, nil
	}
	modelsDevCache = fetched
	startModelsDevRefresh()
	return modelsDevCache, nil
}

func LookupModelsDevModel(providerID string, modelID string) (*ModelsDevProvider, *ModelsDevModel) {
	db, err := LoadModelsDev()
	if err != nil {
		return nil, nil
	}
	provider, ok := db[providerID]
	if !ok {
		return nil, nil
	}
	model, ok := provider.Models[modelID]
	if !ok {
		return &provider, nil
	}
	return &provider, &model
}

func modelCostFromModelsDev(cost *ModelsDevCost) *ModelCost {
	if cost == nil {
		return nil
	}
	out := &ModelCost{
		Input:  cost.Input,
		Output: cost.Output,
		Cache: ModelCacheCost{
			Read:  cost.CacheRead,
			Write: cost.CacheWrite,
		},
	}
	if cost.ContextOver200K != nil {
		out.ExperimentalOver200K = &ModelCost{
			Input:  cost.ContextOver200K.Input,
			Output: cost.ContextOver200K.Output,
			Cache: ModelCacheCost{
				Read:  cost.ContextOver200K.CacheRead,
				Write: cost.ContextOver200K.CacheWrite,
			},
		}
	}
	return out
}

func fetchModelsDev(cachePath string) (map[string]ModelsDevProvider, error) {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(global.ModelsDevURL() + "/api.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{ProviderID: "models.dev", Message: "models.dev request failed", StatusCode: resp.StatusCode}
	}
	var parsed map[string]ModelsDevProvider
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if data, err := json.Marshal(parsed); err == nil {
		_ = os.WriteFile(cachePath, data, 0o644)
	}
	return parsed, nil
}

func RefreshModelsDev() error {
	if os.Getenv(global.EnvDisableModelsFetch) != "" {
		return nil
	}
	if err := global.Init(); err != nil {
		return err
	}
	cachePath := filepath.Join(global.Path.Cache, "models.json")
	fetched, err := fetchModelsDev(cachePath)
	if err != nil {
		return err
	}
	modelsDevMu.Lock()
	modelsDevCache = fetched
	modelsDevLoaded = true
	modelsDevMu.Unlock()
	return nil
}

func startModelsDevRefresh() {
	if os.Getenv(global.EnvDisableModelsFetch) != "" {
		return
	}
	modelsDevRefreshOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				if os.Getenv(global.EnvDisableModelsFetch) != "" {
					return
				}
				_ = RefreshModelsDev()
				<-ticker.C
			}
		}()
	})
}
