package diagnostics

import (
	"go.lsp.dev/protocol"
)

const (
	PhpStanProviderId   string = "phpstan"
	PhpStanProviderName string = "PHPStan Static Analysis"
)

type PhpStan struct{}

func (dp *PhpStan) Id() string {
	return PhpStanProviderId
}

func (dp *PhpStan) Name() string {
	return PhpStanProviderName
}

func (dp *PhpStan) Analyze(filePath string) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic
	diagnostics = mockDiagnostics(dp, diagnostics)

	return diagnostics, nil
}

func NewPhpStan() *PhpStan {
	return &PhpStan{}
}
