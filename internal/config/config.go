package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	Name           string = "php-diagls"
	Version        string = "0.0.2"
	ConfigFileName string = ".php-diagls.json"

	ConfigItemDiagnosticsProviders string = "diagnosticsProviders"
)

type Config struct {
	RawData              json.RawMessage
	DiagnosticsProviders map[string]DiagnosticsProvider
	initialized          bool
}

type FormatConfig struct {
	Enabled bool `json:"enabled"`
}

type DiagnosticsProvider struct {
	Enabled    bool         `json:"enabled"`
	Container  string       `json:"container"`
	Path       string       `json:"path"`
	ConfigFile string       `json:"configFile"`
	Format     FormatConfig `json:"format"`
}

func (config *Config) IsInitialized() bool {
	return config.initialized
}

func (config *Config) LoadConfig(projectRoot string) (*Config, error) {
	configPath := filepath.Join(projectRoot, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return config, fmt.Errorf("config file not found: %s", configPath)
	}

	rawData, err := os.ReadFile(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	rawMap := make(map[string]json.RawMessage)
	if err := json.Unmarshal(rawData, &rawMap); err != nil {
		return config, fmt.Errorf("failed to parse config file: %w", err)
	}

	diagnosticsProvidersData := make(map[string]DiagnosticsProvider)
	if rawProviders, exists := rawMap[ConfigItemDiagnosticsProviders]; exists {
		if err := json.Unmarshal(rawProviders, &diagnosticsProvidersData); err != nil {
			return config, fmt.Errorf("failed to parse diagnostics providers: %w", err)
		}
	} else {
		return config, fmt.Errorf("no diagnostics providers configured (missing key %s)", ConfigItemDiagnosticsProviders)
	}

	config.RawData = rawData
	config.DiagnosticsProviders = diagnosticsProvidersData
	config.initialized = true

	return config, nil
}
