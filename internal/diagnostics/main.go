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

type FormattingProvider interface {
	Format(filePath string) (string, error)
}

func NewDiagnosticsProvider(providerId string, providerConfig config.DiagnosticsProvider) (DiagnosticsProvider, error) {
	runner := container.NewDockerCommandRunner()
	err := validateProviderConfig(runner, providerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize %s; error: %s", providerId, err)
	}

	switch providerId {
	case PhpCsFixerProviderId:
		return NewPhpCsFixer(providerConfig, runner), nil
	case PhpStanProviderId:
		return NewPhpStan(providerConfig, runner), nil
	default:
		return nil, fmt.Errorf("unknown diagnostics provider: %s", providerId)
	}
}

func validateProviderConfig(runner container.CommandRunner, providerConfig config.DiagnosticsProvider) error {
	err := container.ValidateContainer(providerConfig.Container)
	if err != nil {
		return err
	}

	err = container.ValidateBinaryInContainer(runner, providerConfig.Container, providerConfig.Path)
	if err != nil {
		return err
	}

	return nil
}