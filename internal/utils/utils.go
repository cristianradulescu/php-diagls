package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"go.lsp.dev/protocol"
)

func URIToPath(uri protocol.DocumentURI) string {
	return strings.TrimPrefix(string(uri), "file://")
}

// Find the project root directory by looking for the config file
func FindProjectRoot(filePath string) string {
	dir := filepath.Dir(filePath)

	for {
		configPath := filepath.Join(dir, config.ConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	// If no config found, use the directory of the file
	return filepath.Dir(filePath)
}

func EnsureDiagnosticsArray(diagnostics []protocol.Diagnostic) []protocol.Diagnostic {
	if diagnostics == nil {
		return make([]protocol.Diagnostic, 0)
	}
	return diagnostics
}

func SnakeCaseToHumanReadable(stringToConvert string) string {
	stringToConvert = strings.Trim(stringToConvert, "_")
	if stringToConvert == "" {
		return ""
	}

	parts := strings.Split(stringToConvert, "_")
	runes := []rune(parts[0])
	runes[0] = unicode.ToUpper(runes[0])
	parts[0] = string(runes)

	return strings.Join(parts, " ")

}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// ApplyUnifiedDiff applies a unified diff to the original content to produce the modified content
func ApplyUnifiedDiff(originalContent, diff string) (string, error) {
	lines := strings.Split(originalContent, "\n")
	diffLines := strings.Split(diff, "\n")

	result := make([]string, 0, len(lines))
	originalLineNum := 0

	re := regexp.MustCompile(`@@\s+-(\d+),(\d+)?\s+\+(\d+),(\d+)?\s+@@`)

	i := 0
	for i < len(diffLines) {
		line := diffLines[i]

		// Skip diff header lines
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			i++
			continue
		}

		// Handle hunk header
		if strings.HasPrefix(line, "@@") {
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 2 {
				if startLine, err := strconv.Atoi(matches[1]); err == nil {
					// Copy lines before this hunk
					for originalLineNum < startLine-1 && originalLineNum < len(lines) {
						result = append(result, lines[originalLineNum])
						originalLineNum++
					}
				}
			}
			i++
			continue
		}

		// Handle diff content
		if len(line) == 0 {
			i++
			continue
		}

		switch line[0] {
		case ' ':
			// Context line - copy from original
			if originalLineNum < len(lines) {
				result = append(result, lines[originalLineNum])
				originalLineNum++
			}
		case '-':
			// Removed line - skip it in original
			if originalLineNum < len(lines) {
				originalLineNum++
			}
		case '+':
			// Added line - add to result
			result = append(result, line[1:])
		}

		i++
	}

	// Copy any remaining lines from original
	for originalLineNum < len(lines) {
		result = append(result, lines[originalLineNum])
		originalLineNum++
	}

	return strings.Join(result, "\n"), nil
}
