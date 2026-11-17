package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Secrets maps app type id → credential name → value.
// File is optional: /config/secrets.yaml
// Example:
//
//	homeassistant:
//	  key: "..."
//	pihole:
//	  key: "..."
type Secrets map[string]map[string]string

func LoadSecrets(configDir string) Secrets {
	path := filepath.Join(configDir, "secrets.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return Secrets{}
	}
	expanded := expandEnv(string(raw))
	var s Secrets
	if err := yaml.Unmarshal([]byte(expanded), &s); err != nil {
		return Secrets{}
	}
	if s == nil {
		return Secrets{}
	}
	return s
}

func (s Secrets) ForApp(appType string) map[string]string {
	if s == nil {
		return map[string]string{}
	}
	out := map[string]string{}
	if m, ok := s[appType]; ok {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}
