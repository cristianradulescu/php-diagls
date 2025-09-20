package utils_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/utils"
	"go.lsp.dev/protocol"
)

func TestURIToPath(t *testing.T) {
	tests := []struct {
		name     string
		uri      protocol.DocumentURI
		expected string
	}{
		{
			name:     "standard file URI",
			uri:      "file:///home/user/project/file.php",
			expected: "/home/user/project/file.php",
		},
		{
			name:     "file URI with spaces",
			uri:      "file:///home/user/my%20project/file.php",
			expected: "/home/user/my%20project/file.php",
		},
		{
			name:     "Windows file URI",
			uri:      "file:///C:/Users/user/project/file.php",
			expected: "/C:/Users/user/project/file.php",
		},
		{
			name:     "relative path URI",
			uri:      "file://./file.php",
			expected: "./file.php",
		},
		{
			name:     "URI without file prefix",
			uri:      "/home/user/file.php",
			expected: "/home/user/file.php",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.URIToPath(tt.uri)
			if result != tt.expected {
				t.Errorf("URIToPath(%s) = %s; expected %s", tt.uri, result, tt.expected)
			}
		})
	}
}

func TestFindProjectRoot(t *testing.T) {
	// Create temporary directory structure
	tempDir := t.TempDir()

	// Create nested directory structure
	projectRoot := filepath.Join(tempDir, "project")
	subDir := filepath.Join(projectRoot, "src", "deep", "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create config file in project root
	configPath := filepath.Join(projectRoot, config.ConfigFileName)
	configContent := `{"diagnosticsProviders": {}}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	tests := []struct {
		name         string
		filePath     string
		expectedRoot string
	}{
		{
			name:         "file in project root",
			filePath:     filepath.Join(projectRoot, "main.php"),
			expectedRoot: projectRoot,
		},
		{
			name:         "file in subdirectory",
			filePath:     filepath.Join(projectRoot, "src", "file.php"),
			expectedRoot: projectRoot,
		},
		{
			name:         "file in deep nested directory",
			filePath:     filepath.Join(subDir, "file.php"),
			expectedRoot: projectRoot,
		},
		{
			name:         "file outside project",
			filePath:     filepath.Join(tempDir, "outside.php"),
			expectedRoot: tempDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.FindProjectRoot(tt.filePath)
			if result != tt.expectedRoot {
				t.Errorf("FindProjectRoot(%s) = %s; expected %s", tt.filePath, result, tt.expectedRoot)
			}
		})
	}
}

func TestFindProjectRoot_NoConfig(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.php")
	expectedRoot := tempDir

	result := utils.FindProjectRoot(filePath)
	if result != expectedRoot {
		t.Errorf("FindProjectRoot(%s) = %s; expected %s (file directory when no config found)",
			filePath, result, expectedRoot)
	}
}

func TestEnsureDiagnosticsArray(t *testing.T) {
	tests := []struct {
		name     string
		input    []protocol.Diagnostic
		expected []protocol.Diagnostic
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: make([]protocol.Diagnostic, 0),
		},
		{
			name:     "empty slice",
			input:    []protocol.Diagnostic{},
			expected: []protocol.Diagnostic{},
		},
		{
			name: "non-empty slice",
			input: []protocol.Diagnostic{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 0, Character: 10},
					},
					Message:  "Test diagnostic",
					Severity: protocol.DiagnosticSeverityError,
				},
			},
			expected: []protocol.Diagnostic{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: 0, Character: 10},
					},
					Message:  "Test diagnostic",
					Severity: protocol.DiagnosticSeverityError,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.EnsureDiagnosticsArray(tt.input)

			if result == nil {
				t.Error("EnsureDiagnosticsArray should never return nil")
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected length %d, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i].Message != expected.Message {
					t.Errorf("Diagnostic %d: expected message %s, got %s",
						i, expected.Message, result[i].Message)
				}
				if result[i].Severity != expected.Severity {
					t.Errorf("Diagnostic %d: expected severity %v, got %v", i, expected.Severity, result[i].Severity)
				}
			}
		})
	}
}

func TestSnakeCaseToHumanReadable(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple snake_case",
			input:    "php_cs_fixer",
			expected: "Php cs fixer",
		},
		{
			name:     "single word",
			input:    "phpstan",
			expected: "Phpstan",
		},
		{
			name:     "multiple underscores",
			input:    "very_long_snake_case_name",
			expected: "Very long snake case name",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "string without underscores",
			input:    "alreadynormal",
			expected: "Alreadynormal",
		},
		{
			name:     "leading underscore",
			input:    "_private_method",
			expected: "Private method",
		},
		{
			name:     "trailing underscore",
			input:    "method_name_",
			expected: "Method name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.SnakeCaseToHumanReadable(tt.input)
			if result != tt.expected {
				t.Errorf("SnakeCaseToHumanReadable(%s) = %s; expected %s",
					tt.input, result, tt.expected)
			}
		})
	}
}
