package server

import (
	"github.com/cristianradulescu/php-diagls/internal/config"
	"go.lsp.dev/protocol"
)

func mockDiagnostics(diagnostics []protocol.Diagnostic) []protocol.Diagnostic {
	diagnostics = append(
		diagnostics,
		protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 0},
			},
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   config.Name,
			Message:  "[TEST] Code style violation on first line",
		},
		protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: 3, Character: 0},
				End:   protocol.Position{Line: 3, Character: 0},
			},
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   config.Name,
			Message:  "[TEST] Code style violation one line 4",
		},
	)

	return diagnostics
}
