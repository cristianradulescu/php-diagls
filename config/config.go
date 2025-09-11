package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	Name           string = "php-diagls"
	Version        string = "0.0.1-dev"
	ConfigFileName string = ".php-diagls.json"
)

type Config struct {
	RawData              json.RawMessage
	DiagnosticsProviders map[string]DiagnosticsProvider
}

type DiagnosticsProvider struct {
	Enabled    bool   `json:"enabled"`
	Container  string `json:"container"`
	Path       string `json:"path"`
	ConfigFile string `json:"configFile"`
}

func LoadConfig(projectRoot string) (*Config, error) {
	configPath := filepath.Join(projectRoot, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{}, fmt.Errorf("config file not found: %s", configPath)
	}

	rawData, err := os.ReadFile(configPath)
	if err != nil {
		return &Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	rawMap := make(map[string]json.RawMessage)
	if err := json.Unmarshal(rawData, &rawMap); err != nil {
		return &Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	diagnosticsProvidersData := make(map[string]DiagnosticsProvider)
	if rawProviders, exists := rawMap["diagnosticsProviders"]; exists {
		if err := json.Unmarshal(rawProviders, &diagnosticsProvidersData); err != nil {
			return &Config{}, fmt.Errorf("failed to parse diagnostics providers: %w", err)
		}
	}

	return &Config{
		RawData:              rawData,
		DiagnosticsProviders: diagnosticsProvidersData,
	}, nil
}
