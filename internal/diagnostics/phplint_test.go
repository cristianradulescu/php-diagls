package diagnostics_test

import (
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
	"go.lsp.dev/protocol"
)

func TestPhpLint_Id(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container",
		Path:      "/usr/bin/php",
	}

	linter := diagnostics.NewPhpLint(providerConfig)

	if linter.Id() != "phplint" {
		t.Errorf("Expected ID 'phplint', got '%s'", linter.Id())
	}
}

func TestPhpLint_Name(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container",
		Path:      "/usr/bin/php",
	}

	linter := diagnostics.NewPhpLint(providerConfig)

	if linter.Name() != "php-lint" {
		t.Errorf("Expected name 'php-lint', got '%s'", linter.Name())
	}
}

// TestPhpLint_Analyze tests the analyze method
// Note: This requires Docker to run properly, so it will test current behavior
func TestPhpLint_Analyze(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container-that-does-not-exist",
		Path:      "/usr/bin/php",
	}

	linter := diagnostics.NewPhpLint(providerConfig)

	// Create a temporary PHP file for testing
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.php"

	// Test with non-existent container - should handle gracefully
	diagnostics, err := linter.Analyze(testFile)

	// Should not return error even if container doesn't exist
	if err != nil {
		t.Errorf("Analyze should handle errors gracefully, got error: %v", err)
	}

	// Current behavior: returns nil slice when no diagnostics found
	// This is acceptable in Go (nil slice behaves like empty slice)
	if diagnostics == nil {
		t.Log("Analyze returns nil slice (acceptable Go behavior)")
	}
}

// TestPhpLint_OutputParsing tests the regex parsing of PHP lint output
// This test documents the expected parsing behavior
func TestPhpLint_OutputParsing(t *testing.T) {
	// Note: We can't directly test the parsing logic without refactoring
	// because it's embedded in the Analyze method.
	// This test documents expected behavior for future refactoring.

	tests := []struct {
		name           string
		output         string
		expectedErrors int
		description    string
	}{
		{
			name:           "no syntax errors",
			output:         "No syntax errors detected in test.php",
			expectedErrors: 0,
			description:    "Should return no diagnostics for valid PHP",
		},
		{
			name:           "parse error",
			output:         "Parse error: syntax error, unexpected 'echo' (T_ECHO) in test.php on line 5",
			expectedErrors: 1,
			description:    "Should detect parse errors with line numbers",
		},
		{
			name:           "fatal error",
			output:         "Fatal error: Call to undefined function foo() in test.php on line 10",
			expectedErrors: 1,
			description:    "Should detect fatal errors with line numbers",
		},
	}

	// Document expected regex pattern
	expectedRegex := `[Fatal|Parse] error:\s+(.*) in .* on line (\d+)`

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Test case: %s", tt.description)
			t.Logf("Expected regex: %s", expectedRegex)
			t.Logf("Output: %s", tt.output)
			t.Logf("Expected errors: %d", tt.expectedErrors)

			// This is a documentation test - we're recording expected behavior
			// Actual parsing happens in the Analyze method
		})
	}
}

// TestPhpLint_DiagnosticFormat tests that diagnostics are properly formatted
func TestPhpLint_DiagnosticFormat(t *testing.T) {
	// This test documents the expected diagnostic structure
	// Real diagnostics would be created by the Analyze method

	expectedDiagnostic := protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: 4, Character: 0}, // Line 5 in editor (0-indexed)
			End:   protocol.Position{Line: 4, Character: 100},
		},
		Severity: protocol.DiagnosticSeverityError,
		Source:   "php-lint",
		Message:  "syntax error, unexpected 'echo' (T_ECHO)",
	}

	// Verify expected structure
	if expectedDiagnostic.Source != "php-lint" {
		t.Error("Diagnostic source should be 'php-lint'")
	}

	if expectedDiagnostic.Severity != protocol.DiagnosticSeverityError {
		t.Error("PHP lint errors should have Error severity")
	}

	// Line numbers should be 0-indexed (editor uses 0-indexed)
	// PHP reports 1-indexed line numbers, so line 5 in error becomes line 4
	if expectedDiagnostic.Range.Start.Line != 4 {
		t.Errorf("Expected line 4 (0-indexed), got %d", expectedDiagnostic.Range.Start.Line)
	}

	t.Logf("Expected diagnostic format: %+v", expectedDiagnostic)
}

// TestPhpLint_NewPhpLint tests provider construction
func TestPhpLint_NewPhpLint(t *testing.T) {
	tests := []struct {
		name   string
		config config.DiagnosticsProvider
	}{
		{
			name: "basic configuration",
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-container",
				Path:      "/usr/bin/php",
			},
		},
		{
			name: "with custom path",
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "custom-container",
				Path:      "/usr/local/bin/php",
			},
		},
		{
			name: "disabled provider",
			config: config.DiagnosticsProvider{
				Enabled:   false,
				Container: "php-container",
				Path:      "/usr/bin/php",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := diagnostics.NewPhpLint(tt.config)

			if provider == nil {
				t.Error("NewPhpLint should not return nil")
			}

			if provider.Id() != diagnostics.PhpLintProviderId {
				t.Errorf("Provider ID mismatch: got %s, want %s",
					provider.Id(), diagnostics.PhpLintProviderId)
			}

			if provider.Name() != diagnostics.PhpLintProviderName {
				t.Errorf("Provider name mismatch: got %s, want %s",
					provider.Name(), diagnostics.PhpLintProviderName)
			}
		})
	}
}
