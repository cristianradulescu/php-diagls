package diagnostics_test

import (
	"errors"
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/container"
	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
	"go.lsp.dev/protocol"
)

func TestPhpLint_Analyze(t *testing.T) {
	tests := []struct {
		name              string
		commandOutput     []byte
		commandError      error
		expectedError     bool
		expectedErrorContains string
		expectedDiagnostics []protocol.Diagnostic
	}{
		{
			name:              "no syntax errors",
			commandOutput:     []byte("No syntax errors detected in /path/to/file.php"),
			commandError:      nil,
			expectedError:     false,
			expectedDiagnostics: []protocol.Diagnostic{},
		},
		{
			name:          "syntax error",
			commandOutput: []byte("PHP Parse error:  syntax error, unexpected 'echo' (T_ECHO), expecting ',' or ';' in /path/to/file.php on line 5"),
			commandError:  nil,
			expectedError: false,
			expectedDiagnostics: []protocol.Diagnostic{
				{
					Range:    protocol.Range{Start: protocol.Position{Line: 4, Character: 0}, End: protocol.Position{Line: 4, Character: 100}},
					Severity: protocol.DiagnosticSeverityError,
					Source:   diagnostics.PhpLintProviderName,
					Message:  "syntax error, unexpected 'echo' (T_ECHO), expecting ',' or ';'",
				},
			},
		},
		{
			name:          "command error",
			commandOutput: []byte("some error"),
			commandError:  errors.New("command failed"),
			expectedError: false,
			expectedDiagnostics: []protocol.Diagnostic{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the container command
			originalRunCommand := container.RunCommandInContainer
			container.RunCommandInContainer = func(containerName string, containerCmd string) ([]byte, error) {
				return tt.commandOutput, tt.commandError
			}
			defer func() { container.RunCommandInContainer = originalRunCommand }()

			provider := diagnostics.NewPhpLint(config.DiagnosticsProvider{
				Container: "test-container",
				Path:      "/usr/bin/php",
			})

			diagnostics, err := provider.Analyze("/path/to/file.php")

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if len(diagnostics) != len(tt.expectedDiagnostics) {
				t.Errorf("Expected %d diagnostics, but got %d", len(tt.expectedDiagnostics), len(diagnostics))
			}

			for i, expected := range tt.expectedDiagnostics {
				actual := diagnostics[i]
				if actual.Range != expected.Range {
					t.Errorf("Expected range %v, but got %v", expected.Range, actual.Range)
				}
				if actual.Severity != expected.Severity {
					t.Errorf("Expected severity %v, but got %v", expected.Severity, actual.Severity)
				}
				if actual.Source != expected.Source {
					t.Errorf("Expected source %s, but got %s", expected.Source, actual.Source)
				}
				if actual.Message != expected.Message {
					t.Errorf("Expected message '%s', but got '%s'", expected.Message, actual.Message)
				}
			}
		})
	}
}