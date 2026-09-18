package diagnostics

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/container"
	"github.com/cristianradulescu/php-diagls/internal/utils"
	"go.lsp.dev/protocol"
)

const (
	PhpLintProviderId   string = "phplint"
	PhpLintProviderName string = "php-lint"
)

// syntaxErrorRegexp matches php -l's "Fatal error: ..."/"Parse error: ..."
// output. Uses a non-capturing alternation, not a character class, so it
// requires the literal word "Fatal" or "Parse" rather than any single
// character drawn from those letters.
var syntaxErrorRegexp = regexp.MustCompile(`(?:Fatal|Parse) error:\s+(.*) in .* on line (\d+)`)

type PhpLint struct {
	config config.DiagnosticsProvider
}

func (dp *PhpLint) Id() string {
	return PhpLintProviderId
}

func (dp *PhpLint) Name() string {
	return PhpLintProviderName
}

func (dp *PhpLint) Analyze(ctx context.Context, filePath string) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	projectRoot := utils.FindProjectRoot(filePath)
	relativeFilePath, _ := filepath.Rel(projectRoot, filePath)

	if utils.IsPathExcluded(relativeFilePath, dp.config.ExcludePaths) {
		return diagnostics, nil
	}

	result := container.RunCommandInContainer(
		ctx,
		dp.config.Container,
		fmt.Sprintf("%s -l %s 2>&1", utils.ShellQuote(dp.config.Path), utils.ShellQuote(relativeFilePath)),
	)

	return dp.diagnosticsFromOutput(string(result.Stdout), result)
}

// diagnosticsFromOutput turns `php -l` output into diagnostics. A syntax
// error yields exactly one diagnostic (php -l stops at the first); a clean
// run yields none. Anything else with a failing exit code is reported as an
// error so a broken container or binary doesn't masquerade as a clean file.
func (dp *PhpLint) diagnosticsFromOutput(output string, result *container.CommandResult) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	if strings.Contains(output, "No syntax errors detected") {
		return diagnostics, nil
	}

	matches := syntaxErrorRegexp.FindStringSubmatch(output)

	if len(matches) == 3 {
		line, convErr := strconv.Atoi(matches[2])
		if convErr != nil {
			return diagnostics, convErr
		}
		if line > 0 {
			line--
		}

		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range:    protocol.Range{Start: protocol.Position{Line: uint32(line), Character: 0}, End: protocol.Position{Line: uint32(line), Character: 100}},
			Severity: protocol.DiagnosticSeverityError,
			Source:   dp.Name(),
			Message:  strings.TrimSpace(matches[1]),
		})
		return diagnostics, nil
	}

	if result.Err != nil {
		return diagnostics, fmt.Errorf("running php -l: %w", result.Err)
	}
	if result.ExitCode != 0 {
		return diagnostics, fmt.Errorf("php -l failed (exit %d): %s", result.ExitCode, utils.SummarizeOutput([]byte(output), nil))
	}

	return diagnostics, nil
}

func NewPhpLint(providerConfig config.DiagnosticsProvider) *PhpLint {
	return &PhpLint{
		config: providerConfig,
	}
}
