package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

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

func (dp *PhpStan) Analyze(ctx context.Context, filePath string) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	projectRoot := utils.FindProjectRoot(filePath)
	relativeFilePath, _ := filepath.Rel(projectRoot, filePath)

	if utils.IsPathExcluded(relativeFilePath, dp.config.ExcludePaths) {
		return diagnostics, nil
	}

	configArg := ""
	if dp.config.ConfigFile != "" {
		configArg = "--configuration=" + utils.ShellQuote(dp.config.ConfigFile)
	}
	result := container.RunCommandInContainer(
		ctx,
		dp.config.Container,
		fmt.Sprintf("%s analyze %s --memory-limit=-1 --no-progress --error-format=json %s", utils.ShellQuote(dp.config.Path), utils.ShellQuote(relativeFilePath), configArg),
	)

	if result.Err != nil {
		return []protocol.Diagnostic{}, fmt.Errorf("running phpstan: %w", result.Err)
	}

	// phpstan exits 1 when it reports errors but still prints the JSON
	// report; output that doesn't parse means it never got that far.
	var report PhpstanOutputResult
	if err := json.Unmarshal(result.Stdout, &report); err != nil {
		return []protocol.Diagnostic{}, fmt.Errorf("phpstan produced no report (exit %d): %s", result.ExitCode, utils.SummarizeOutput(result.Stderr, result.Stdout))
	}

	return dp.diagnosticsFromReport(report)
}

// diagnosticsFromReport converts a parsed phpstan JSON report into LSP
// diagnostics. Non-file errors (bad config, missing autoloader, ...) come
// back in the top-level "errors" array and are returned as an error
// alongside whatever file diagnostics were produced.
func (dp *PhpStan) diagnosticsFromReport(report PhpstanOutputResult) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	for _, file := range report.Files {
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

	if len(report.Errors) > 0 {
		return diagnostics, fmt.Errorf("phpstan reported: %s", strings.Join(report.Errors, "; "))
	}

	return diagnostics, nil
}

func NewPhpStan(providerConfig config.DiagnosticsProvider) *PhpStan {
	return &PhpStan{
		config: providerConfig,
	}
}
