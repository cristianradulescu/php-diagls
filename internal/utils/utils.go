package utils

import (
	"os"
	"path/filepath"
	"strings"

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
