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

func TestCopyFile(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (src, dst string)
		wantErr bool
		verify  func(t *testing.T, src, dst string)
	}{
		{
			name: "copy valid file",
			setup: func(t *testing.T) (src, dst string) {
				t.Helper()
				tmpDir := t.TempDir()

				// Create source file
				src = filepath.Join(tmpDir, "source.txt")
				content := "Hello, World!\nThis is a test file."
				if err := os.WriteFile(src, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create source file: %v", err)
				}

				// Destination path
				dst = filepath.Join(tmpDir, "destination.txt")
				return src, dst
			},
			wantErr: false,
			verify: func(t *testing.T, src, dst string) {
				t.Helper()
				// Verify destination file exists
				if _, err := os.Stat(dst); os.IsNotExist(err) {
					t.Error("Destination file was not created")
					return
				}

				// Verify content matches
				srcContent, err := os.ReadFile(src)
				if err != nil {
					t.Fatalf("Failed to read source: %v", err)
				}
				dstContent, err := os.ReadFile(dst)
				if err != nil {
					t.Fatalf("Failed to read destination: %v", err)
				}

				if string(srcContent) != string(dstContent) {
					t.Errorf("Content mismatch.\nSource: %s\nDestination: %s",
						string(srcContent), string(dstContent))
				}
			},
		},
		{
			name: "source file does not exist",
			setup: func(t *testing.T) (src, dst string) {
				t.Helper()
				tmpDir := t.TempDir()
				src = filepath.Join(tmpDir, "nonexistent.txt")
				dst = filepath.Join(tmpDir, "destination.txt")
				return src, dst
			},
			wantErr: true,
			verify:  nil,
		},
		{
			name: "destination directory does not exist",
			setup: func(t *testing.T) (src, dst string) {
				t.Helper()
				tmpDir := t.TempDir()

				// Create source file
				src = filepath.Join(tmpDir, "source.txt")
				if err := os.WriteFile(src, []byte("test"), 0644); err != nil {
					t.Fatalf("Failed to create source file: %v", err)
				}

				// Destination in non-existent directory
				dst = filepath.Join(tmpDir, "nonexistent", "destination.txt")
				return src, dst
			},
			wantErr: true,
			verify:  nil,
		},
		{
			name: "copy preserves content exactly",
			setup: func(t *testing.T) (src, dst string) {
				t.Helper()
				tmpDir := t.TempDir()

				// Create source with various content
				src = filepath.Join(tmpDir, "source.txt")
				content := "Line 1\nLine 2\n\nLine 4 with special chars: !@#$%^&*()\nUnicode: 你好世界 🎉"
				if err := os.WriteFile(src, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create source file: %v", err)
				}

				dst = filepath.Join(tmpDir, "destination.txt")
				return src, dst
			},
			wantErr: false,
			verify: func(t *testing.T, src, dst string) {
				t.Helper()
				srcContent, _ := os.ReadFile(src)
				dstContent, _ := os.ReadFile(dst)

				// Byte-by-byte comparison
				if len(srcContent) != len(dstContent) {
					t.Errorf("File sizes differ: src=%d, dst=%d", len(srcContent), len(dstContent))
				}

				for i := 0; i < len(srcContent) && i < len(dstContent); i++ {
					if srcContent[i] != dstContent[i] {
						t.Errorf("Byte mismatch at position %d: src=%d, dst=%d",
							i, srcContent[i], dstContent[i])
						break
					}
				}
			},
		},
		{
			name: "copy empty file",
			setup: func(t *testing.T) (src, dst string) {
				t.Helper()
				tmpDir := t.TempDir()

				src = filepath.Join(tmpDir, "empty.txt")
				if err := os.WriteFile(src, []byte(""), 0644); err != nil {
					t.Fatalf("Failed to create empty file: %v", err)
				}

				dst = filepath.Join(tmpDir, "empty-copy.txt")
				return src, dst
			},
			wantErr: false,
			verify: func(t *testing.T, src, dst string) {
				t.Helper()
				dstContent, err := os.ReadFile(dst)
				if err != nil {
					t.Fatalf("Failed to read destination: %v", err)
				}
				if len(dstContent) != 0 {
					t.Errorf("Expected empty file, got %d bytes", len(dstContent))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, dst := tt.setup(t)

			err := utils.CopyFile(src, dst)

			// Check error expectation
			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Run verification if provided and no error expected
			if !tt.wantErr && tt.verify != nil {
				tt.verify(t, src, dst)
			}
		})
	}
}
