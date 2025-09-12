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

