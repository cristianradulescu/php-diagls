package diagnostics

import (
	"fmt"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/container"
	"go.lsp.dev/protocol"
)

type DiagnosticsProvider interface {
	Id() string
	Name() string
	Analyze(filePath string) ([]protocol.Diagnostic, error)
}

func NewDiagnosticsProvider(providerId string, providerConfig config.DiagnosticsProvider) (DiagnosticsProvider, error) {
	err := validateProviderConfig(providerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize %s; error: %s", providerId, err)
	}

	switch providerId {
	case PhpCsFixerProviderId:
		return NewPhpCsFixer(providerConfig), nil
	case PhpStanProviderId:
		return NewPhpStan(), nil
	default:
		return nil, fmt.Errorf("unknown diagnostics provider: %s", providerId)
	}
}

func validateProviderConfig(providerConfig config.DiagnosticsProvider) error {
	err := container.ValidateContainer(providerConfig.Container)
	if err != nil {
		return err
	}

	err = container.ValidateBinaryInContainer(providerConfig.Container, providerConfig.Path)
	if err != nil {
		return err
	}

	return nil
}
