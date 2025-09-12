package diagnostics

import (
	"go.lsp.dev/protocol"
)

type PhpStan struct {
	enabled bool
}

func (dp *PhpStan) Id() string {
	return "phpstan"
}

func (dp *PhpStan) Name() string {
	return "PHPStan Static Analysis Tool"
}

func (dp *PhpStan) IsEnabled() bool {
	return dp.enabled
}

func (dp *PhpStan) SetEnabled(enabled bool) {
	dp.enabled = enabled
}

func (dp *PhpStan) Analyze(filePath string) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic
	diagnostics = mockDiagnostics(dp, diagnostics)

	return diagnostics, nil
}
