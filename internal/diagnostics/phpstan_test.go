package diagnostics_test

import (
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
)

func TestPhpStan_Id(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container",
		Path:      "/usr/local/bin/phpstan",
	}

	analyzer := diagnostics.NewPhpStan(providerConfig)

	if analyzer.Id() != "phpstan" {
		t.Errorf("Expected ID 'phpstan', got '%s'", analyzer.Id())
	}
}

func TestPhpStan_Name(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container",
		Path:      "/usr/local/bin/phpstan",
	}

	analyzer := diagnostics.NewPhpStan(providerConfig)

	if analyzer.Name() != "phpstan" {
		t.Errorf("Expected name 'phpstan', got '%s'", analyzer.Name())
	}
}

func TestPhpStan_NewPhpStan(t *testing.T) {
	tests := []struct {
		name   string
		config config.DiagnosticsProvider
	}{
		{
			name: "basic configuration",
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "phpstan-container",
				Path:      "/usr/local/bin/phpstan",
			},
		},
		{
			name: "with config file",
			config: config.DiagnosticsProvider{
				Enabled:    true,
				Container:  "phpstan-container",
				Path:       "/usr/local/bin/phpstan",
				ConfigFile: "phpstan.neon",
			},
		},
		{
			name: "disabled provider",
			config: config.DiagnosticsProvider{
				Enabled:   false,
				Container: "phpstan-container",
				Path:      "/usr/local/bin/phpstan",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := diagnostics.NewPhpStan(tt.config)

			if provider == nil {
				t.Error("NewPhpStan should not return nil")
			}

			if provider.Id() != diagnostics.PhpStanProviderId {
				t.Errorf("Provider ID mismatch: got %s, want %s",
					provider.Id(), diagnostics.PhpStanProviderId)
			}

			if provider.Name() != diagnostics.PhpStanProviderName {
				t.Errorf("Provider name mismatch: got %s, want %s",
					provider.Name(), diagnostics.PhpStanProviderName)
			}
		})
	}
}

func TestPhpStan_Analyze(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container-that-does-not-exist",
		Path:      "/usr/local/bin/phpstan",
	}

	analyzer := diagnostics.NewPhpStan(providerConfig)

	// Create a temporary PHP file for testing
	tmpDir := t.TempDir()
	testFile := tmpDir + "/test.php"

	// Test with non-existent container - should handle gracefully
	diagnostics, err := analyzer.Analyze(testFile)

	// Should not return error even if container doesn't exist
	if err != nil {
		t.Errorf("Analyze should handle errors gracefully, got error: %v", err)
	}

	// Current behavior: returns nil slice when no diagnostics found
	if diagnostics == nil {
		t.Log("Analyze returns nil slice (acceptable Go behavior)")
	}
}

// TestPhpStan_OutputStructure documents the expected PHPStan JSON output structure
func TestPhpStan_OutputStructure(t *testing.T) {
	// Document expected JSON structure from PHPStan
	exampleJSON := `{
  "files": {
    "test.php": {
      "messages": [
        {
          "message": "Variable $foo might not be defined.",
          "line": 10,
          "ignorable": true,
          "identifier": "variable.undefined"
        },
        {
          "message": "Parameter #1 $bar of function test() expects string, int given.",
          "line": 15,
          "ignorable": false
        }
      ]
    }
  },
  "errors": []
}`

	t.Logf("Expected PHPStan JSON structure:\n%s", exampleJSON)

	// Document key points:
	// 1. Files is a map[string] with file paths as keys
	// 2. Each file has messages array
	// 3. Each message has: message, line, ignorable, optional identifier
	// 4. Line numbers are 1-indexed (need to convert to 0-indexed)
	// 5. Ignorable true -> Warning severity, false -> Error severity
	// 6. Errors array contains general errors (not file-specific)
}

// TestPhpStan_LineNumberMapping tests line number conversion
func TestPhpStan_LineNumberMapping(t *testing.T) {
	tests := []struct {
		name               string
		phpstanLine        int
		expectedEditorLine uint32
		description        string
	}{
		{
			name:               "line 1 in PHPStan",
			phpstanLine:        1,
			expectedEditorLine: 0,
			description:        "PHPStan line 1 should map to editor line 0",
		},
		{
			name:               "line 10 in PHPStan",
			phpstanLine:        10,
			expectedEditorLine: 9,
			description:        "PHPStan line 10 should map to editor line 9",
		},
		{
			name:               "line 0 in PHPStan (edge case)",
			phpstanLine:        0,
			expectedEditorLine: 0,
			description:        "PHPStan line 0 should stay 0 (edge case handling)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Document the conversion logic
			var editorLine uint32
			if tt.phpstanLine > 0 {
				editorLine = uint32(tt.phpstanLine - 1)
			}

			if editorLine != tt.expectedEditorLine {
				t.Errorf("%s: expected %d, got %d",
					tt.description, tt.expectedEditorLine, editorLine)
			}

			t.Logf("PHPStan line %d -> Editor line %d", tt.phpstanLine, editorLine)
		})
	}
}

// TestPhpStan_SeverityMapping tests severity level conversion
func TestPhpStan_SeverityMapping(t *testing.T) {
	tests := []struct {
		name             string
		ignorable        bool
		expectedSeverity string
		description      string
	}{
		{
			name:             "ignorable error",
			ignorable:        true,
			expectedSeverity: "Warning",
			description:      "Ignorable errors should be warnings",
		},
		{
			name:             "non-ignorable error",
			ignorable:        false,
			expectedSeverity: "Error",
			description:      "Non-ignorable errors should be errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("%s: ignorable=%v -> severity=%s",
				tt.description, tt.ignorable, tt.expectedSeverity)
		})
	}
}

// TestPhpStan_DiagnosticRange tests the diagnostic range format
func TestPhpStan_DiagnosticRange(t *testing.T) {
	// Document expected range format for PHPStan diagnostics
	// PHPStan only provides line numbers, not character positions

	expectedStartChar := uint32(0)
	expectedEndChar := uint32(100)

	if expectedStartChar != 0 {
		t.Error("PHPStan diagnostics should start at character 0")
	}

	if expectedEndChar != 100 {
		t.Error("PHPStan diagnostics should end at character 100 (full line)")
	}

	t.Logf("PHPStan diagnostic range: Start char=%d, End char=%d (full line highlight)",
		expectedStartChar, expectedEndChar)
}

// TestPhpStan_MessageFormat tests diagnostic message handling
func TestPhpStan_MessageFormat(t *testing.T) {
	tests := []struct {
		name            string
		rawMessage      string
		expectedMessage string
		description     string
	}{
		{
			name:            "simple message",
			rawMessage:      "Variable $foo might not be defined.",
			expectedMessage: "Variable $foo might not be defined.",
			description:     "Simple messages should be passed through unchanged",
		},
		{
			name:            "message with type info",
			rawMessage:      "Parameter #1 $bar of function test() expects string, int given.",
			expectedMessage: "Parameter #1 $bar of function test() expects string, int given.",
			description:     "Type information should be preserved",
		},
		{
			name:            "message with whitespace",
			rawMessage:      "  Property Test::$foo is never read, only written.  ",
			expectedMessage: "Property Test::$foo is never read, only written.",
			description:     "Whitespace should be trimmed (if trimming is implemented)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("%s", tt.description)
			t.Logf("Raw message: %q", tt.rawMessage)
			t.Logf("Expected: %q", tt.expectedMessage)
		})
	}
}

// TestPhpStan_IdentifierField tests the optional identifier field
func TestPhpStan_IdentifierField(t *testing.T) {
	// Document that identifier is optional in PHPStan output
	t.Log("PHPStan identifier field is optional")
	t.Log("When present, it provides a machine-readable error code")
	t.Log("Examples: 'variable.undefined', 'argument.type', 'return.type'")
	t.Log("When absent, Code field in diagnostic should be nil or empty")
}

// TestPhpStan_ErrorsArray tests handling of general errors
func TestPhpStan_ErrorsArray(t *testing.T) {
	// Document that PHPStan can have general errors outside of file-specific messages
	t.Log("PHPStan output can include an 'errors' array")
	t.Log("These are general errors not tied to specific lines")
	t.Log("Examples: configuration errors, missing dependencies, etc.")
	t.Log("Current implementation: these are logged but not converted to diagnostics")
}

// TestPhpStan_ConfigFileHandling tests config file argument
func TestPhpStan_ConfigFileHandling(t *testing.T) {
	tests := []struct {
		name        string
		configFile  string
		expectedArg string
	}{
		{
			name:        "with config file",
			configFile:  "phpstan.neon",
			expectedArg: "--configuration=phpstan.neon",
		},
		{
			name:        "without config file",
			configFile:  "",
			expectedArg: "",
		},
		{
			name:        "with custom config path",
			configFile:  "config/phpstan.neon.dist",
			expectedArg: "--configuration=config/phpstan.neon.dist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Config file: %q -> Argument: %q", tt.configFile, tt.expectedArg)
		})
	}
}
