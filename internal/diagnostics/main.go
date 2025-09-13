package diagnostics

import (
	"go.lsp.dev/protocol"
)

type DiagnosticsProvider interface {
	Id() string
	Name() string
	IsEnabled() bool
	SetEnabled(enabled bool)
	Analyze(filePath string) ([]protocol.Diagnostic, error)
}

func NewDiagnosticsProvider(providerId string) DiagnosticsProvider {
	switch providerId {
	case PhpCsFixerProviderId:
		return NewPhpCsFixer()
	case PhpStanProviderId:
		return NewPhpStan()
	default:
		return nil
	}
}

