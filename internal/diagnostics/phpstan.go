package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/container"
	"github.com/cristianradulescu/php-diagls/internal/utils"
	"go.lsp.dev/protocol"
)

const (
	PhpStanProviderId   string = "phpstan"
	PhpStanProviderName string = "phpstan"
)

type PhpstanOutputResult struct {
	Files map[string]struct {
		Messages []struct {
			Message    string  `json:"message"`
			Line       int     `json:"line"`
			Ignorable  bool    `json:"ignorable"`
			Identifier *string `json:"identifier,omitempty"`
		} `json:"messages"`
	} `json:"files"`
	Errors []string `json:"errors"`
}

type PhpStan struct {
	config config.DiagnosticsProvider
}

func (dp *PhpStan) Id() string {
	return PhpStanProviderId
}

func (dp *PhpStan) Name() string {
	return PhpStanProviderName
}

func (dp *PhpStan) Analyze(filePath string) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	projectRoot := utils.FindProjectRoot(filePath)
	relativeFilePath, _ := filepath.Rel(projectRoot, filePath)

	configArg := ""
	if dp.config.ConfigFile != "" {
		configArg = fmt.Sprintf("--configuration=%s", dp.config.ConfigFile)
	}
	fullAnalysisCmdOutput, _ := container.RunCommandInContainer(
		context.Background(),
		dp.config.Container,
		fmt.Sprintf("%s analyze %s --memory-limit=-1 --no-progress --error-format=json %s 2>/dev/null", dp.config.Path, relativeFilePath, configArg),
	)
	// log.Printf("Full analysis cmd result: %s", string(fullAnalysisCmdOutput))

	var fullAnalysisResult PhpstanOutputResult
	if err := json.Unmarshal(fullAnalysisCmdOutput, &fullAnalysisResult); err != nil {
		log.Printf("Unmarshall err: %s", err)
		return []protocol.Diagnostic{}, nil
	}

	for _, file := range fullAnalysisResult.Files {
		for _, message := range file.Messages {
			line := uint32(0)
			if message.Line > 0 {
				line = uint32(message.Line - 1)
			}

			severity := protocol.DiagnosticSeverityError
			if message.Ignorable {
				severity = protocol.DiagnosticSeverityWarning
			}

			diagnostic := protocol.Diagnostic{
				Range:    protocol.Range{Start: protocol.Position{Line: line, Character: 0}, End: protocol.Position{Line: line, Character: 100}},
				Severity: severity,
				Source:   dp.Name(),
				Message:  message.Message,
			}
			if message.Identifier != nil {
				diagnostic.Code = *message.Identifier
			}
			diagnostics = append(diagnostics, diagnostic)
		}
	}

	return diagnostics, nil
}

func NewPhpStan(providerConfig config.DiagnosticsProvider) *PhpStan {
	return &PhpStan{
		config: providerConfig,
	}
}
