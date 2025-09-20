package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/config"
)

func TestConfig_LoadConfig(t *testing.T) {
	tests := []struct {
		name           string
		configContent  string
		expectedError  bool
		errorContains  string
		expectedConfig *config.Config
	}{
		{
			name: "valid config with phpcsfixer",
			configContent: `{
				"diagnosticsProviders": {
					"phpcsfixer": {
						"enabled": true,
						"container": "my-php-container",
						"path": "/usr/local/bin/php-cs-fixer",
						"configFile": ".php-cs-fixer.dist.php"
					}
				}
			}`,
			expectedError: false,
			expectedConfig: &config.Config{
				DiagnosticsProviders: map[string]config.DiagnosticsProvider{
					"phpcsfixer": {
						Enabled:    true,
						Container:  "my-php-container",
						Path:       "/usr/local/bin/php-cs-fixer",
						ConfigFile: ".php-cs-fixer.dist.php",
					},
				},
			},
		},
		{
			name: "valid config with multiple providers",
			configContent: `{
				"diagnosticsProviders": {
					"phpcsfixer": {
						"enabled": true,
						"container": "my-php-container",
						"path": "/usr/local/bin/php-cs-fixer",
						"configFile": ".php-cs-fixer.dist.php"
					},
					"phpstan": {
						"enabled": false,
						"container": "my-php-container",
						"path": "/usr/local/bin/phpstan",
						"configFile": "phpstan.neon"
					}
				}
			}`,
			expectedError: false,
			expectedConfig: &config.Config{
				DiagnosticsProviders: map[string]config.DiagnosticsProvider{
					"phpcsfixer": {
						Enabled:    true,
						Container:  "my-php-container",
						Path:       "/usr/local/bin/php-cs-fixer",
						ConfigFile: ".php-cs-fixer.dist.php",
					},
					"phpstan": {
						Enabled:    false,
						Container:  "my-php-container",
						Path:       "/usr/local/bin/phpstan",
						ConfigFile: "phpstan.neon",
					},
				},
			},
		},
		{
			name: "missing diagnosticsProviders key",
			configContent: `{
				"otherConfig": "value"
			}`,
			expectedError: true,
			errorContains: "no diagnostics providers configured",
		},
		{
			name: "invalid JSON",
			configContent: `{
				"diagnosticsProviders": {
					"phpcsfixer": {
						"enabled": true,
						"container": "my-php-container",
						"path": "/usr/local/bin/php-cs-fixer"
					}
				},  // invalid trailing comma
			}`,
			expectedError: true,
			errorContains: "failed to parse config file",
		},
		{
			name: "invalid diagnosticsProviders format",
			configContent: `{
				"diagnosticsProviders": "invalid-string-instead-of-object"
			}`,
			expectedError: true,
			errorContains: "failed to parse diagnostics providers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory and config file
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, config.ConfigFileName)

			if err := os.WriteFile(configPath, []byte(tt.configContent), 0644); err != nil {
				t.Fatalf("Failed to create test config file: %v", err)
			}

			// Test loading config
			cfg := &config.Config{}
			result, err := cfg.LoadConfig(tempDir)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !result.IsInitialized() {
				t.Error("Config should be initialized after successful load")
			}

			// Verify diagnostics providers
			if len(result.DiagnosticsProviders) != len(tt.expectedConfig.DiagnosticsProviders) {
				t.Errorf("Expected %d providers, got %d",
					len(tt.expectedConfig.DiagnosticsProviders),
					len(result.DiagnosticsProviders))
			}

			for id, expectedProvider := range tt.expectedConfig.DiagnosticsProviders {
				actualProvider, exists := result.DiagnosticsProviders[id]
				if !exists {
					t.Errorf("Expected provider %s not found", id)
					continue
				}

				if actualProvider.Enabled != expectedProvider.Enabled {
					t.Errorf("Provider %s: expected enabled=%v, got %v",
						id, expectedProvider.Enabled, actualProvider.Enabled)
				}
				if actualProvider.Container != expectedProvider.Container {
					t.Errorf("Provider %s: expected container=%s, got %s",
						id, expectedProvider.Container, actualProvider.Container)
				}
				if actualProvider.Path != expectedProvider.Path {
					t.Errorf("Provider %s: expected path=%s, got %s",
						id, expectedProvider.Path, actualProvider.Path)
				}
				if actualProvider.ConfigFile != expectedProvider.ConfigFile {
					t.Errorf("Provider %s: expected configFile=%s, got %s",
						id, expectedProvider.ConfigFile, actualProvider.ConfigFile)
				}
			}

			// Verify raw data is stored
			if len(result.RawData) == 0 {
				t.Error("Expected RawData to be populated")
			}
		})
	}
}

func TestConfig_LoadConfig_FileNotFound(t *testing.T) {
	cfg := &config.Config{}
	_, err := cfg.LoadConfig("/non/existent/path")

	if err == nil {
		t.Error("Expected error for non-existent config file")
	}

	if !containsString(err.Error(), "config file not found") {
		t.Errorf("Expected 'config file not found' error, got: %s", err.Error())
	}
}

func TestConfig_IsInitialized(t *testing.T) {
	cfg := &config.Config{}

	if cfg.IsInitialized() {
		t.Error("New config should not be initialized")
	}

	// Create a valid config file and load it
	tempDir := t.TempDir()
	configContent := `{
		"diagnosticsProviders": {
			"phpcsfixer": {
				"enabled": true,
				"container": "test-container",
				"path": "/bin/php-cs-fixer",
				"configFile": ".php-cs-fixer.php"
			}
		}
	}`
	configPath := filepath.Join(tempDir, config.ConfigFileName)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	_, err := cfg.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if !cfg.IsInitialized() {
		t.Error("Config should be initialized after successful load")
	}
}

func TestConstants(t *testing.T) {
	if config.Name == "" {
		t.Error("Name constant should not be empty")
	}
	if config.Version == "" {
		t.Error("Version constant should not be empty")
	}
	if config.ConfigFileName == "" {
		t.Error("ConfigFileName constant should not be empty")
	}
	if config.ConfigItemDiagnosticsProviders == "" {
		t.Error("ConfigItemDiagnosticsProviders constant should not be empty")
	}

	// Test expected values
	expectedName := "php-diagls"
	expectedConfigFile := ".php-diagls.json"
	expectedDiagnosticsKey := "diagnosticsProviders"

	if config.Name != expectedName {
		t.Errorf("Expected Name=%s, got %s", expectedName, config.Name)
	}
	if config.ConfigFileName != expectedConfigFile {
		t.Errorf("Expected ConfigFileName=%s, got %s", expectedConfigFile, config.ConfigFileName)
	}
	if config.ConfigItemDiagnosticsProviders != expectedDiagnosticsKey {
		t.Errorf("Expected ConfigItemDiagnosticsProviders=%s, got %s", expectedDiagnosticsKey, config.ConfigItemDiagnosticsProviders)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && (s == substr || containsString(s[1:], substr) || (len(s) > 0 && s[:len(substr)] == substr))
}
