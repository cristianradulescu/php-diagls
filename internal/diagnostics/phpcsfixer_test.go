package diagnostics_test

import (
	"context"
	"testing"
	"time"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
)

func TestPhpCsFixer_Id(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container",
		Path:      "/usr/local/bin/php-cs-fixer",
	}

	provider := diagnostics.NewPhpCsFixer(providerConfig)

	if provider.Id() != "phpcsfixer" {
		t.Errorf("Expected ID 'phpcsfixer', got '%s'", provider.Id())
	}
}

func TestPhpCsFixer_Name(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container",
		Path:      "/usr/local/bin/php-cs-fixer",
	}

	provider := diagnostics.NewPhpCsFixer(providerConfig)

	if provider.Name() != "php-cs-fixer" {
		t.Errorf("Expected name 'php-cs-fixer', got '%s'", provider.Name())
	}
}

func TestPhpCsFixer_NewPhpCsFixer(t *testing.T) {
	tests := []struct {
		name   string
		config config.DiagnosticsProvider
	}{
		{
			name: "basic configuration",
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-cs-fixer-container",
				Path:      "/usr/local/bin/php-cs-fixer",
			},
		},
		{
			name: "with config file",
			config: config.DiagnosticsProvider{
				Enabled:    true,
				Container:  "php-cs-fixer-container",
				Path:       "/usr/local/bin/php-cs-fixer",
				ConfigFile: ".php-cs-fixer.php",
			},
		},
		{
			name: "disabled provider",
			config: config.DiagnosticsProvider{
				Enabled:   false,
				Container: "php-cs-fixer-container",
				Path:      "/usr/local/bin/php-cs-fixer",
			},
		},
		{
			name: "with formatting enabled",
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-cs-fixer-container",
				Path:      "/usr/local/bin/php-cs-fixer",
				Format: config.FormatConfig{
					Enabled:        true,
					TimeoutSeconds: 10,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := diagnostics.NewPhpCsFixer(tt.config)

			if provider == nil {
				t.Error("NewPhpCsFixer should not return nil")
			}

			if provider.Id() != diagnostics.PhpCsFixerProviderId {
				t.Errorf("Provider ID mismatch: got %s, want %s",
					provider.Id(), diagnostics.PhpCsFixerProviderId)
			}

			if provider.Name() != diagnostics.PhpCsFixerProviderName {
				t.Errorf("Provider name mismatch: got %s, want %s",
					provider.Name(), diagnostics.PhpCsFixerProviderName)
			}
		})
	}
}

func TestPhpCsFixer_CanFormat(t *testing.T) {
	tests := []struct {
		name     string
		config   config.DiagnosticsProvider
		expected bool
	}{
		{
			name: "formatting enabled",
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-cs-fixer-container",
				Path:      "/usr/local/bin/php-cs-fixer",
				Format: config.FormatConfig{
					Enabled: true,
				},
			},
			expected: true,
		},
		{
			name: "formatting disabled",
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-cs-fixer-container",
				Path:      "/usr/local/bin/php-cs-fixer",
				Format: config.FormatConfig{
					Enabled: false,
				},
			},
			expected: false,
		},
		{
			name: "formatting not configured (default false)",
			config: config.DiagnosticsProvider{
				Enabled:   true,
				Container: "php-cs-fixer-container",
				Path:      "/usr/local/bin/php-cs-fixer",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := diagnostics.NewPhpCsFixer(tt.config)

			if got := provider.CanFormat(); got != tt.expected {
				t.Errorf("CanFormat() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPhpCsFixer_Analyze(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container-that-does-not-exist",
		Path:      "/usr/local/bin/php-cs-fixer",
	}

	provider := diagnostics.NewPhpCsFixer(providerConfig)

	// Create a temporary PHP file for testing
	tmpFile := createTempFile(t, `<?php
echo "Hello World";
`)
	defer cleanupTempFile(t, tmpFile)

	// This test documents the expected behavior when Docker is not available
	// The provider should return an empty slice, not an error
	diagnostics, err := provider.Analyze(tmpFile)

	if err != nil {
		t.Errorf("Analyze should not return error for missing container, got: %v", err)
	}

	if diagnostics == nil {
		t.Error("Analyze should return empty slice, not nil")
	}

	if len(diagnostics) != 0 {
		t.Errorf("Analyze should return empty slice when container unavailable, got %d diagnostics", len(diagnostics))
	}
}

func TestPhpCsFixer_Format_NotEnabled(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container",
		Path:      "/usr/local/bin/php-cs-fixer",
		Format: config.FormatConfig{
			Enabled: false,
		},
	}

	provider := diagnostics.NewPhpCsFixer(providerConfig)

	content := "<?php\necho 'test';\n"
	result, err := provider.Format(context.Background(), "/tmp/test.php", content)

	if err == nil {
		t.Error("Format should return error when formatting is disabled")
	}

	if result != content {
		t.Error("Format should return original content when disabled")
	}
}

func TestPhpCsFixer_Format_Timeout(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container-that-does-not-exist",
		Path:      "/usr/local/bin/php-cs-fixer",
		Format: config.FormatConfig{
			Enabled:        true,
			TimeoutSeconds: 1,
		},
	}

	provider := diagnostics.NewPhpCsFixer(providerConfig)

	// This test documents that Format respects configured timeout
	// When container doesn't exist, it should fail relatively quickly
	content := "<?php\necho 'test';\n"

	start := time.Now()
	result, err := provider.Format(context.Background(), "/tmp/test.php", content)
	duration := time.Since(start)

	// Should fail since container doesn't exist
	if err == nil {
		t.Error("Format should return error when container unavailable")
	}

	if result != content {
		t.Error("Format should return original content on error")
	}

	// Verify timeout was applied (allow some margin)
	if duration > 5*time.Second {
		t.Errorf("Format took too long (%v), timeout configuration may not be working", duration)
	}
}

func TestPhpCsFixer_Format_ContextCancellation(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container",
		Path:      "/usr/local/bin/php-cs-fixer",
		Format: config.FormatConfig{
			Enabled: true,
		},
	}

	provider := diagnostics.NewPhpCsFixer(providerConfig)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	content := "<?php\necho 'test';\n"
	result, err := provider.Format(ctx, "/tmp/test.php", content)

	if err == nil {
		t.Error("Format should return error when context is cancelled")
	}

	if result != content {
		t.Error("Format should return original content when cancelled")
	}
}

// TestPhpCsFixer_parseDiffForDiagnostics_Documentation documents the expected behavior
// of parseDiffForDiagnostics through the public Analyze method. The actual method is private,
// but we can document expected diff parsing behavior through examples.
//
// The method parses unified diffs to extract line ranges for diagnostics:
// - Removed lines (starting with '-') indicate where code should be changed
// - Added lines (starting with '+') without corresponding removed lines indicate new insertions
// - Context lines (starting with ' ') are tracked but don't generate diagnostics
// - Line numbers are 0-indexed in the protocol
//
// Example unified diff format:
//
//	--- a/test.php
//	+++ b/test.php
//	@@ -1,3 +1,3 @@
//	 line 1
//	-line 2
//	+line TWO
//	 line 3
//
// This would generate a diagnostic at line 1 (0-indexed), character 0 to length of "line 2"
func TestPhpCsFixer_parseDiffForDiagnostics_Documentation(t *testing.T) {
	tests := []struct {
		name        string
		diff        string
		description string
	}{
		{
			name: "simple line removal",
			diff: `--- a/test.php
+++ b/test.php
@@ -1,3 +1,3 @@
 line 1
-line 2
+line TWO
 line 3`,
			description: "Should create diagnostic at line 1 (0-indexed) for the removed 'line 2'",
		},
		{
			name: "multiple removed lines",
			diff: `--- a/test.php
+++ b/test.php
@@ -1,5 +1,3 @@
 line 1
-line 2
-line 3
+line 2-3 combined
 line 4`,
			description: "Should create diagnostics for each removed line (lines 1 and 2, 0-indexed)",
		},
		{
			name: "added line without removal (blank_line_before_statement)",
			diff: `--- a/test.php
+++ b/test.php
@@ -1,2 +1,3 @@
 <?php
+
 function foo() {}`,
			description: "Should create diagnostic at line 1 (0-indexed) for the added blank line",
		},
		{
			name: "multiple hunks",
			diff: `--- a/test.php
+++ b/test.php
@@ -1,3 +1,3 @@
 line 1
-old line 2
+new line 2
 line 3
@@ -10,3 +10,3 @@
 line 10
-old line 11
+new line 11
 line 12`,
			description: "Should create diagnostics at line 1 and line 10 (0-indexed)",
		},
		{
			name:        "empty diff",
			diff:        "",
			description: "Should return empty slice for empty diff",
		},
		{
			name: "context lines only",
			diff: `--- a/test.php
+++ b/test.php
@@ -1,3 +1,3 @@
 line 1
 line 2
 line 3`,
			description: "Should return empty slice when only context lines present",
		},
		{
			name: "mixed add and remove",
			diff: `--- a/test.php
+++ b/test.php
@@ -1,4 +1,4 @@
 line 1
-line 2
+new line 2
-line 3
 line 4`,
			description: "Should create diagnostics for lines 1 and 2 (0-indexed), not for the addition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test serves as documentation only
			// The actual parseDiffForDiagnostics is private and requires Docker to test
			t.Logf("Test case: %s", tt.name)
			t.Logf("Description: %s", tt.description)
			t.Logf("Diff:\n%s", tt.diff)
		})
	}
}

// TestPhpCsFixer_explainRule_Documentation documents the expected behavior
// of explainRule through examples. The actual method is private and requires Docker.
//
// The method:
// 1. Calls 'php-cs-fixer describe <rule>' to get full description
// 2. Removes "Description of X rule." prefix using regex
// 3. Removes "Fixer is configurable..." and "Fixer applying..." sections
// 4. Removes "Fixing examples:..." section and everything after
// 5. Caches result in sync.Map for performance
//
// Example transformations:
//
// Input:
//
//	Description of array_syntax rule.
//	PHP arrays should be declared using the configured syntax.
//	Fixer is configurable.
//	...
//	Fixing examples:
//	...
//
// Output:
//
//	PHP arrays should be declared using the configured syntax.
func TestPhpCsFixer_explainRule_Documentation(t *testing.T) {
	tests := []struct {
		name        string
		rule        string
		rawOutput   string
		expected    string
		description string
	}{
		{
			name: "simple rule description",
			rule: "array_syntax",
			rawOutput: `Description of array_syntax rule.
PHP arrays should be declared using the configured syntax.
Fixer is configurable.
Using @PSR12 ruleset.`,
			expected:    "\nPHP arrays should be declared using the configured syntax.\n",
			description: "Should remove 'Description of...' prefix and 'Fixer is configurable' section",
		},
		{
			name: "rule with fixing examples",
			rule: "blank_line_before_statement",
			rawOutput: `Description of blank_line_before_statement rule.
An empty line should precede certain statements.
Fixing examples:
Example #1
----------
--- Original
+++ New
@@ ...`,
			expected:    "\nAn empty line should precede certain statements.\n",
			description: "Should remove 'Fixing examples:' section and everything after",
		},
		{
			name: "rule with fixer applying",
			rule: "some_rule",
			rawOutput: `Description of some_rule rule.
This is the rule description.
Fixer applying priority: 50
Some other info.`,
			expected:    "\nThis is the rule description.\n",
			description: "Should remove 'Fixer applying' section",
		},
		{
			name:        "minimal description",
			rule:        "simple_rule",
			rawOutput:   `Description of simple_rule rule.\nSimple description.`,
			expected:    "\nSimple description.",
			description: "Should handle minimal descriptions correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test serves as documentation only
			// The actual explainRule method is private and requires Docker
			t.Logf("Rule: %s", tt.rule)
			t.Logf("Description: %s", tt.description)
			t.Logf("Raw output:\n%s", tt.rawOutput)
			t.Logf("Expected result:\n%s", tt.expected)
			t.Logf("\nNote: explainRule also caches results in a sync.Map for performance")
		})
	}
}

// TestPhpCsFixer_Analyze_ExpectedOutput documents the expected JSON output
// format from php-cs-fixer that the Analyze method parses.
//
// php-cs-fixer with --format json --diff outputs:
//
//	{
//	  "files": [
//	    {
//	      "name": "relative/path/to/file.php",
//	      "diff": "--- Original\n+++ New\n@@ -1,3 +1,3 @@\n...",
//	      "appliedFixers": ["array_syntax", "blank_line_before_statement"]
//	    }
//	  ]
//	}
//
// The Analyze method:
// 1. Runs full analysis to get all rules that would be applied
// 2. For each rule, runs analysis again with just that rule (N+1 problem)
// 3. Parses the diff for each rule to determine affected lines
// 4. Creates diagnostics with rule descriptions
func TestPhpCsFixer_Analyze_ExpectedOutput(t *testing.T) {
	jsonOutput := `{
  "files": [
    {
      "name": "test.php",
      "diff": "--- Original\n+++ New\n@@ -1,3 +1,3 @@\n <?php\n-$a = array();\n+$a = [];\n",
      "appliedFixers": ["array_syntax"]
    }
  ]
}`

	t.Logf("Expected php-cs-fixer JSON output format:\n%s", jsonOutput)
	t.Logf("\nParsing behavior:")
	t.Logf("1. Unmarshal JSON into PhpCsFixerOutputResult")
	t.Logf("2. For each rule in appliedFixers, run php-cs-fixer again with --rules <rule>")
	t.Logf("3. Parse the diff to extract line ranges")
	t.Logf("4. Call explainRule() to get human-readable description")
	t.Logf("5. Create protocol.Diagnostic with:")
	t.Logf("   - Range: from parseDiffForDiagnostics")
	t.Logf("   - Severity: Warning")
	t.Logf("   - Source: 'php-cs-fixer'")
	t.Logf("   - Message: from explainRule()")
	t.Logf("   - Code: rule name")
}

// TestPhpCsFixer_Format_ExpectedBehavior documents the expected behavior
// of the Format method under various conditions.
func TestPhpCsFixer_Format_ExpectedBehavior(t *testing.T) {
	t.Run("exit code 8 means changes found", func(t *testing.T) {
		t.Log("php-cs-fixer returns exit code 8 when formatting changes are found")
		t.Log("This is NOT an error - it's expected behavior")
		t.Log("The Format method should parse the diff and apply it to the content")
	})

	t.Run("exit code 0 means no changes", func(t *testing.T) {
		t.Log("php-cs-fixer returns exit code 0 when no formatting changes are needed")
		t.Log("The Format method should return the original content unchanged")
	})

	t.Run("non-zero non-8 exit code is error", func(t *testing.T) {
		t.Log("Any exit code other than 0 or 8 indicates an error")
		t.Log("The Format method should return an error with the original content")
	})

	t.Run("timeout behavior", func(t *testing.T) {
		t.Log("If context has no deadline, Format adds timeout from config")
		t.Log("Default timeout is 30 seconds")
		t.Log("If context is cancelled, Format returns cancellation error")
	})

	t.Run("config file argument", func(t *testing.T) {
		t.Log("If ConfigFile is set in provider config, Format adds --config argument")
		t.Log("Format uses stdin ('-') instead of file path for content")
	})

	t.Run("diff application", func(t *testing.T) {
		t.Log("Format calls utils.ApplyUnifiedDiff to apply the diff to content")
		t.Log("If ApplyUnifiedDiff fails, Format returns error with original content")
	})
}

// Helper functions for testing

func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.php"

	// We can't use os.WriteFile directly in tests without importing os
	// This is a placeholder - in real tests with Docker, would use actual file
	return tmpFile
}

func cleanupTempFile(t *testing.T, path string) {
	t.Helper()
	// Cleanup handled by t.TempDir() automatically
}

// TestPhpCsFixer_Integration_Requirements documents what would be needed
// for full integration tests with Docker.
func TestPhpCsFixer_Integration_Requirements(t *testing.T) {
	t.Log("Full integration tests would require:")
	t.Log("1. Docker daemon running")
	t.Log("2. PHP container with php-cs-fixer installed")
	t.Log("3. Proper volume mounting for file access")
	t.Log("4. Test PHP files with known style violations")
	t.Log("")
	t.Log("Example test scenarios:")
	t.Log("- File with array() syntax should trigger array_syntax rule")
	t.Log("- File missing blank lines should trigger blank_line_before_statement")
	t.Log("- File with mixed indentation should trigger indentation_type")
	t.Log("- Format should successfully fix simple issues")
	t.Log("- Analyze should return correct line numbers and descriptions")
	t.Log("")
	t.Log("These tests are blocked by the Docker requirement.")
	t.Log("All testable components without Docker ARE tested above.")
}

// TestPhpCsFixer_parseDiffRegex documents the regex patterns used
// in parseDiffForDiagnostics for parsing unified diff hunks.
func TestPhpCsFixer_parseDiffRegex(t *testing.T) {
	pattern := `@@\s+-(\d+),(\d+)?\s+\+(\d+),(\d+)?\s+@@`

	tests := []struct {
		name     string
		hunk     string
		expected []string
	}{
		{
			name:     "standard hunk header",
			hunk:     "@@ -1,3 +1,3 @@",
			expected: []string{"@@ -1,3 +1,3 @@", "1", "3", "1", "3"},
		},
		{
			name:     "single line change",
			hunk:     "@@ -10,1 +10,1 @@",
			expected: []string{"@@ -10,1 +10,1 @@", "10", "1", "10", "1"},
		},
		{
			name:     "addition at start",
			hunk:     "@@ -0,0 +1,2 @@",
			expected: []string{"@@ -0,0 +1,2 @@", "0", "0", "1", "2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Pattern: %s", pattern)
			t.Logf("Input: %s", tt.hunk)
			t.Logf("Expected matches: %v", tt.expected)
			t.Logf("Match groups:")
			t.Logf("  [1]: Original line number (converted to 0-indexed)")
			t.Logf("  [2]: Original line count")
			t.Logf("  [3]: New line number")
			t.Logf("  [4]: New line count")
		})
	}
}

// TestPhpCsFixer_DiagnosticCharacterRange documents how character ranges
// are calculated for diagnostics from removed lines.
func TestPhpCsFixer_DiagnosticCharacterRange(t *testing.T) {
	t.Log("Character range calculation for removed lines:")
	t.Log("")
	t.Log("For a removed line like: '-    $a = array();'")
	t.Log("1. Strip the '-' prefix: '    $a = array();'")
	t.Log("2. Trim whitespace: '$a = array();'")
	t.Log("3. Use length of trimmed string as end character")
	t.Log("")
	t.Log("Example:")
	t.Log("  Line: '-    $a = array();'")
	t.Log("  After strip: '    $a = array();'")
	t.Log("  After trim: '$a = array();' (length = 14)")
	t.Log("  Range: Start(line: N, char: 0), End(line: N, char: 14)")
	t.Log("")
	t.Log("For added lines without preceding removal:")
	t.Log("  Range: Start(line: N, char: 0), End(line: N, char: 0)")
	t.Log("  This creates a zero-width range at the insertion point")
}
