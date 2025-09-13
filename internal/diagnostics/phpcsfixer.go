package diagnostics

import (
	"go.lsp.dev/protocol"
)

const (
	PhpCsFixerProviderId string = "phpcsfixer"
	PhpCsFixerProviderName string = "PHP Coding Standards Fixer"
)

type PhpCsFixer struct {
	enabled bool
}

func (dp *PhpCsFixer) Id() string {
	return PhpCsFixerProviderId
}

func (dp *PhpCsFixer) Name() string {
	return PhpCsFixerProviderName
}

func (dp *PhpCsFixer) IsEnabled() bool {
	return dp.enabled
}

func (dp *PhpCsFixer) SetEnabled(enabled bool) {
	dp.enabled = enabled
}

func (dp *PhpCsFixer) Analyze(filePath string) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic
	diagnostics = mockDiagnostics(dp, diagnostics)

	return diagnostics, nil
}

// constructor
func NewPhpCsFixer() *PhpCsFixer {
	return &PhpCsFixer{
		enabled: true,
	}
}
