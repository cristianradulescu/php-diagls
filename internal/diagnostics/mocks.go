package diagnostics

import (
	"go.lsp.dev/protocol"
)

func mockDiagnostics(provider DiagnosticsProvider, diagnostics []protocol.Diagnostic) []protocol.Diagnostic {
	diagnostics = append(
		diagnostics,
		protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 0},
			},
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   provider.Id(),
			Message:  "[TEST] Code style violation on first line",
			Code:     "TEST001",
			CodeDescription: &protocol.CodeDescription{
				Href: "https://example.com/rules/TEST001",
			},
		},
		protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: 3, Character: 0},
				End:   protocol.Position{Line: 3, Character: 0},
			},
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   provider.Id(),
			Message:  "[TEST] Code style violation one line 4",
			Code:     "TEST004",
			CodeDescription: &protocol.CodeDescription{
				Href: "https://example.com/rules/TEST004",
			},
		},
	)

	return diagnostics
}
