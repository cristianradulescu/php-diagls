package diagnostics

import (
	"go.lsp.dev/protocol"
)

type PhpCsFixer struct {
	enabled bool
}

func (dp *PhpCsFixer) Id() string {
	return "phpcsfixer"
}

func (dp *PhpCsFixer) Name() string {
	return "PHP Coding Standards Fixer"
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
