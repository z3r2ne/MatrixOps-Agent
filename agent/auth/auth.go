package auth

import (
	"encoding/json"
	"os"
	"path/filepath"

	"matrixops-agent/global"
)

const OAuthDummyKey = "matrixops-agent-oauth-dummy-key"

type Info struct {
	Type          string `json:"type"`
	Refresh       string `json:"refresh,omitempty"`
	Access        string `json:"access,omitempty"`
	Expires       int64  `json:"expires,omitempty"`
	AccountID     string `json:"accountId,omitempty"`
	EnterpriseURL string `json:"enterpriseUrl,omitempty"`
	Key           string `json:"key,omitempty"`
	Token         string `json:"token,omitempty"`
}

func Get(providerID string) (*Info, error) {
	all, err := All()
	if err != nil {
		return nil, err
	}
	if info, ok := all[providerID]; ok {
		return &info, nil
	}
	return nil, nil
}

func All() (map[string]Info, error) {
	if err := global.Init(); err != nil {
		return nil, err
	}
	path := filepath.Join(global.Path.Data, "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]Info{}, nil
	}
	var raw map[string]Info
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string]Info{}, nil
	}
	result := make(map[string]Info, len(raw))
	for key, value := range raw {
		switch value.Type {
		case "oauth", "api", "wellknown":
			result[key] = value
		}
	}
	return result, nil
}

func Set(providerID string, info Info) error {
	if err := global.Init(); err != nil {
		return err
	}
	path := filepath.Join(global.Path.Data, "auth.json")
	data, _ := All()
	data[providerID] = info
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func Remove(providerID string) error {
	if err := global.Init(); err != nil {
		return err
	}
	path := filepath.Join(global.Path.Data, "auth.json")
	data, _ := All()
	delete(data, providerID)
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}
