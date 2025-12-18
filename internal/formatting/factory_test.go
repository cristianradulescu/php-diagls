package formatting_test

import (
	"context"
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
	"github.com/cristianradulescu/php-diagls/internal/formatting"
)

func TestNewFormattingProvider_PhpCsFixer(t *testing.T) {
	tests := []struct {
		name        string
		providerId  string
		config      config.DiagnosticsProvider
		expectError bool
		errorMsg    string
	}{
		{
			name:       "php-cs-fixer with formatting enabled",
			providerId: diagnostics.PhpCsFixerProviderId,
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-container",
				Path:      "/usr/local/bin/php-cs-fixer",
				Format: config.FormatConfig{
					Enabled: true,
				},
			},
			expectError: false,
		},
		{
			name:       "php-cs-fixer with formatting disabled",
			providerId: diagnostics.PhpCsFixerProviderId,
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-container",
				Path:      "/usr/local/bin/php-cs-fixer",
				Format: config.FormatConfig{
					Enabled: false,
				},
			},
			expectError: true,
			errorMsg:    "formatting is not enabled",
		},
		{
			name:       "php-cs-fixer with timeout configured",
			providerId: diagnostics.PhpCsFixerProviderId,
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-container",
				Path:      "/usr/local/bin/php-cs-fixer",
				Format: config.FormatConfig{
					Enabled:        true,
					TimeoutSeconds: 60,
				},
			},
			expectError: false,
		},
		{
			name:       "php-cs-fixer with config file",
			providerId: diagnostics.PhpCsFixerProviderId,
			config: config.DiagnosticsProvider{
				Enabled:    true,
				Container:  "php-container",
				Path:       "/usr/local/bin/php-cs-fixer",
				ConfigFile: ".php-cs-fixer.php",
				Format: config.FormatConfig{
					Enabled: true,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := formatting.NewFormattingProvider(tt.providerId, tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
				if provider != nil {
					t.Error("Expected provider to be nil when error occurs")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if provider == nil {
					t.Error("Expected provider to be non-nil")
				} else {
					// Verify provider implements the interface correctly
					if provider.Id() != diagnostics.PhpCsFixerProviderId {
						t.Errorf("Expected provider ID '%s', got '%s'",
							diagnostics.PhpCsFixerProviderId, provider.Id())
					}
					if provider.Name() != diagnostics.PhpCsFixerProviderName {
						t.Errorf("Expected provider name '%s', got '%s'",
							diagnostics.PhpCsFixerProviderName, provider.Name())
					}

					// Verify Format method exists and returns expected behavior
					// Since we don't have Docker, we expect it to fail but not panic
					content := "<?php echo 'test';"
					result, err := provider.Format(context.Background(), "/tmp/test.php", content)
					if err == nil {
						t.Log("Format succeeded (Docker available)")
					} else {
						t.Logf("Format failed as expected without Docker: %v", err)
					}
					// Result should either be formatted content or original content on error
					if result == "" {
						t.Error("Format should never return empty string")
					}
				}
			}
		})
	}
}

func TestNewFormattingProvider_UnsupportedProvider(t *testing.T) {
	tests := []struct {
		name       string
		providerId string
		config     config.DiagnosticsProvider
	}{
		{
			name:       "phpstan does not support formatting",
			providerId: "phpstan",
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-container",
				Path:      "/usr/local/bin/phpstan",
				Format: config.FormatConfig{
					Enabled: true,
				},
			},
		},
		{
			name:       "phplint does not support formatting",
			providerId: "phplint",
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-container",
				Path:      "/usr/local/bin/php",
				Format: config.FormatConfig{
					Enabled: true,
				},
			},
		},
		{
			name:       "unknown provider",
			providerId: "unknown-provider",
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-container",
				Path:      "/usr/local/bin/unknown",
				Format: config.FormatConfig{
					Enabled: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := formatting.NewFormattingProvider(tt.providerId, tt.config)

			if err == nil {
				t.Error("Expected error for unsupported provider")
			}

			if provider != nil {
				t.Error("Expected provider to be nil for unsupported provider")
			}

			if !contains(err.Error(), "not supported") {
				t.Errorf("Expected error message to contain 'not supported', got: %v", err)
			}
		})
	}
}

func TestLoadFormattingProviders(t *testing.T) {
	tests := []struct {
		name                  string
		diagnosticsProviders  map[string]config.DiagnosticsProvider
		expectedProviderCount int
		expectedProviderIds   []string
	}{
		{
			name: "single provider with formatting enabled",
			diagnosticsProviders: map[string]config.DiagnosticsProvider{
				diagnostics.PhpCsFixerProviderId: {
					Enabled:   true,
					Container: "php-container",
					Path:      "/usr/local/bin/php-cs-fixer",
					Format: config.FormatConfig{
						Enabled: true,
					},
				},
			},
			expectedProviderCount: 1,
			expectedProviderIds:   []string{diagnostics.PhpCsFixerProviderId},
		},
		{
			name: "provider disabled",
			diagnosticsProviders: map[string]config.DiagnosticsProvider{
				diagnostics.PhpCsFixerProviderId: {
					Enabled:   false,
					Container: "php-container",
					Path:      "/usr/local/bin/php-cs-fixer",
					Format: config.FormatConfig{
						Enabled: true,
					},
				},
			},
			expectedProviderCount: 0,
			expectedProviderIds:   []string{},
		},
		{
			name: "formatting disabled",
			diagnosticsProviders: map[string]config.DiagnosticsProvider{
				diagnostics.PhpCsFixerProviderId: {
					Enabled:   true,
					Container: "php-container",
					Path:      "/usr/local/bin/php-cs-fixer",
					Format: config.FormatConfig{
						Enabled: false,
					},
				},
			},
			expectedProviderCount: 0,
			expectedProviderIds:   []string{},
		},
		{
			name: "multiple providers, only one supports formatting",
			diagnosticsProviders: map[string]config.DiagnosticsProvider{
				diagnostics.PhpCsFixerProviderId: {
					Enabled:   true,
					Container: "php-container",
					Path:      "/usr/local/bin/php-cs-fixer",
					Format: config.FormatConfig{
						Enabled: true,
					},
				},
				"phpstan": {
					Enabled:   true,
					Container: "php-container",
					Path:      "/usr/local/bin/phpstan",
					Format: config.FormatConfig{
						Enabled: true, // Even if enabled, phpstan doesn't support formatting
					},
				},
			},
			expectedProviderCount: 1,
			expectedProviderIds:   []string{diagnostics.PhpCsFixerProviderId},
		},
		{
			name:                  "empty configuration",
			diagnosticsProviders:  map[string]config.DiagnosticsProvider{},
			expectedProviderCount: 0,
			expectedProviderIds:   []string{},
		},
		{
			name: "mixed enabled and disabled providers",
			diagnosticsProviders: map[string]config.DiagnosticsProvider{
				diagnostics.PhpCsFixerProviderId: {
					Enabled:   true,
					Container: "php-container",
					Path:      "/usr/local/bin/php-cs-fixer",
					Format: config.FormatConfig{
						Enabled: true,
					},
				},
				"phplint": {
					Enabled:   false,
					Container: "php-container",
					Path:      "/usr/local/bin/php",
				},
			},
			expectedProviderCount: 1,
			expectedProviderIds:   []string{diagnostics.PhpCsFixerProviderId},
		},
		{
			name: "provider without format config (disabled by default)",
			diagnosticsProviders: map[string]config.DiagnosticsProvider{
				diagnostics.PhpCsFixerProviderId: {
					Enabled:   true,
					Container: "php-container",
					Path:      "/usr/local/bin/php-cs-fixer",
					// Format not specified - defaults to disabled
				},
			},
			expectedProviderCount: 0,
			expectedProviderIds:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers := formatting.LoadFormattingProviders(tt.diagnosticsProviders)

			if len(providers) != tt.expectedProviderCount {
				t.Errorf("Expected %d providers, got %d", tt.expectedProviderCount, len(providers))
			}

			// Verify all expected provider IDs are present
			foundIds := make(map[string]bool)
			for _, provider := range providers {
				foundIds[provider.Id()] = true
			}

			for _, expectedId := range tt.expectedProviderIds {
				if !foundIds[expectedId] {
					t.Errorf("Expected provider ID '%s' not found in loaded providers", expectedId)
				}
			}

			// Verify no unexpected providers were loaded
			if len(foundIds) != len(tt.expectedProviderIds) {
				t.Errorf("Found unexpected providers: got %d providers, expected %d",
					len(foundIds), len(tt.expectedProviderIds))
			}
		})
	}
}

func TestLoadFormattingProviders_ProvidersAreUsable(t *testing.T) {
	diagnosticsProviders := map[string]config.DiagnosticsProvider{
		diagnostics.PhpCsFixerProviderId: {
			Enabled:   true,
			Container: "php-container",
			Path:      "/usr/local/bin/php-cs-fixer",
			Format: config.FormatConfig{
				Enabled:        true,
				TimeoutSeconds: 30,
			},
		},
	}

	providers := formatting.LoadFormattingProviders(diagnosticsProviders)

	if len(providers) != 1 {
		t.Fatalf("Expected 1 provider, got %d", len(providers))
	}

	provider := providers[0]

	// Verify the provider is fully functional
	if provider.Id() != diagnostics.PhpCsFixerProviderId {
		t.Errorf("Expected provider ID '%s', got '%s'",
			diagnostics.PhpCsFixerProviderId, provider.Id())
	}

	if provider.Name() != diagnostics.PhpCsFixerProviderName {
		t.Errorf("Expected provider name '%s', got '%s'",
			diagnostics.PhpCsFixerProviderName, provider.Name())
	}

	// Verify Format method is callable (even if it fails without Docker)
	content := "<?php echo 'test';"
	result, err := provider.Format(context.Background(), "/tmp/test.php", content)

	// Without Docker, we expect an error, but the provider should handle it gracefully
	if err == nil {
		t.Log("Format succeeded (Docker available)")
		if result == "" {
			t.Error("Format should not return empty string on success")
		}
	} else {
		t.Logf("Format failed as expected without Docker: %v", err)
		// On error, should return original content
		if result != content {
			t.Errorf("Expected original content on error, got different content")
		}
	}
}

// TestFormattingProvider_InterfaceCompliance verifies that the FormattingProvider
// interface is correctly implemented by the supported providers.
func TestFormattingProvider_InterfaceCompliance(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "php-container",
		Path:      "/usr/local/bin/php-cs-fixer",
		Format: config.FormatConfig{
			Enabled: true,
		},
	}

	provider, err := formatting.NewFormattingProvider(diagnostics.PhpCsFixerProviderId, providerConfig)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Verify interface methods exist and return expected types
	t.Run("Id method", func(t *testing.T) {
		id := provider.Id()
		if id == "" {
			t.Error("Id() should return non-empty string")
		}
	})

	t.Run("Name method", func(t *testing.T) {
		name := provider.Name()
		if name == "" {
			t.Error("Name() should return non-empty string")
		}
	})

	t.Run("Format method", func(t *testing.T) {
		// Should not panic when called
		content := "<?php\necho 'test';\n"
		result, err := provider.Format(context.Background(), "/tmp/test.php", content)

		// Result should never be empty (either formatted or original)
		if result == "" && err != nil {
			t.Error("Format should return original content on error, not empty string")
		}

		// Should handle context properly
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		result, err = provider.Format(ctx, "/tmp/test.php", content)
		if err == nil {
			t.Log("Format completed before context cancellation (fast execution)")
		} else {
			if result != content {
				t.Error("Format should return original content when cancelled")
			}
		}
	})
}

// TestNewFormattingProvider_NilConfig documents behavior with nil/empty config.
// This is a documentation test since the actual behavior depends on implementation.
func TestNewFormattingProvider_NilConfig(t *testing.T) {
	t.Log("Testing with minimal/empty configuration")

	config := config.DiagnosticsProvider{
		// Minimal config - Format.Enabled defaults to false
		Enabled:   true,
		Container: "",
		Path:      "",
	}

	provider, err := formatting.NewFormattingProvider(diagnostics.PhpCsFixerProviderId, config)

	// Should fail because formatting is not enabled
	if err == nil {
		t.Error("Expected error when Format.Enabled is false")
	}

	if provider != nil {
		t.Error("Expected nil provider when Format.Enabled is false")
	}

	t.Logf("Error as expected: %v", err)
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
