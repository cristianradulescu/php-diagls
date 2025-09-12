package diagnostics

import (
	"go.lsp.dev/protocol"
)

const (
	providerId string = "phpcsfixer"
	providerName string = "PHP Coding Standards Fixer"
)

type PhpCsFixer struct {
	enabled bool
}

func (dp *PhpCsFixer) Id() string {
	return providerId
}

func (dp *PhpCsFixer) Name() string {
	return providerName
}

func (dp *PhpCsFixer) IsEnabled() bool {
	return dp.enabled
}

func (dp *PhpCsFixer) SetEnabled(enabled bool) {
	dp.enabled = enabled
}

func (dp *PhpCsFixer) Analyze(filePath string) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic
	diagnostics = mockDiagnostics(diagnostics)

	return diagnostics, nil
}
