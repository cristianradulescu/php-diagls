package formatting

import (
	"fmt"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
)

// NewFormattingProvider creates a formatting provider from a diagnostics provider
// if formatting is enabled for that provider
func NewFormattingProvider(providerId string, providerConfig config.DiagnosticsProvider) (FormattingProvider, error) {
	// Only create formatting provider if formatting is enabled
	if !providerConfig.Format.Enabled {
		return nil, fmt.Errorf("formatting is not enabled for provider %s", providerId)
	}

	switch providerId {
	case diagnostics.PhpCsFixerProviderId:
		phpCsFixer := diagnostics.NewPhpCsFixer(providerConfig)
		// Ensure it implements FormattingProvider interface
		if formatter, ok := interface{}(phpCsFixer).(FormattingProvider); ok {
			return formatter, nil
		}
		return nil, fmt.Errorf("provider %s does not implement FormattingProvider interface", providerId)
	default:
		return nil, fmt.Errorf("formatting not supported for provider: %s", providerId)
	}
}

// LoadFormattingProviders creates formatting providers from diagnostics providers configuration
func LoadFormattingProviders(diagnosticsProviders map[string]config.DiagnosticsProvider) []FormattingProvider {
	var providers []FormattingProvider

	for id, providerConfig := range diagnosticsProviders {
		// Skip if provider is not enabled
		if !providerConfig.Enabled {
			continue
		}

		// Skip if formatting is not enabled
		if !providerConfig.Format.Enabled {
			continue
		}

		provider, err := NewFormattingProvider(id, providerConfig)
		if err != nil {
			// Log error but continue with other providers
			continue
		}

		providers = append(providers, provider)
	}

	return providers
}
