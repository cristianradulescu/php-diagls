package diagnostics

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/container"
	"go.lsp.dev/protocol"
)

const (
	PhpLintProviderId   string = "phplint"
	PhpLintProviderName string = "php-lint"
)

type PhpLint struct {
	config config.DiagnosticsProvider
}

func (dp *PhpLint) Id() string {
	return PhpLintProviderId
}

func (dp *PhpLint) Name() string {
	return PhpLintProviderName
}

func (dp *PhpLint) Analyze(filePath string) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	// The -l command does not care about the project root, so we can just use the file path
	// The output also contains the full file path, so no need to compute the relative path
	fullAnalysisCmdOutput, err := container.RunCommandInContainer(
		dp.config.Container,
		fmt.Sprintf("%s -l %s", dp.config.Path, filePath),
	)

	output := string(fullAnalysisCmdOutput)
	if strings.HasPrefix(output, "No syntax errors detected") {
		return diagnostics, nil
	}

	// PHP Parse error:  syntax error, unexpected 'echo' (T_ECHO), expecting ',' or ';' in /path/to/file.php on line 5
	re := regexp.MustCompile(`PHP Parse error:\s+(.*) in .* on line (\d+)`)
	matches := re.FindStringSubmatch(output)

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

	if err != nil {
		log.Printf("Error running phplint command: %v. Output: %s", err, output)
	}

	return diagnostics, nil
}

func NewPhpLint(providerConfig config.DiagnosticsProvider) *PhpLint {
	return &PhpLint{
		config: providerConfig,
	}
}