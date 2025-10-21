package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/container"
	"github.com/cristianradulescu/php-diagls/internal/logging"
	"github.com/cristianradulescu/php-diagls/internal/utils"
	"go.lsp.dev/protocol"
)

const (
	PhpCsFixerProviderId   string = "phpcsfixer"
	PhpCsFixerProviderName string = "php-cs-fixer"
)

type PhpCsFixerOutputResult struct {
	Files []struct {
		Name  string   `json:"name"`
		Diff  string   `json:"diff"`
		Rules []string `json:"appliedFixers"`
	} `json:"files"`
}

type PhpCsFixer struct {
	config           config.DiagnosticsProvider
	ruleDescriptions sync.Map
}

func (dp *PhpCsFixer) Id() string {
	return PhpCsFixerProviderId
}

func (dp *PhpCsFixer) Name() string {
	return PhpCsFixerProviderName
}

func (dp *PhpCsFixer) Analyze(filePath string) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic
	var linesRange []protocol.Range

	projectRoot := utils.FindProjectRoot(filePath)
	relativeFilePath, _ := filepath.Rel(projectRoot, filePath)

	configArg := ""
	if dp.config.ConfigFile != "" {
		configArg = fmt.Sprintf("--config %s", dp.config.ConfigFile)
	}
	result := container.RunCommandInContainer(
		context.Background(),
		dp.config.Container,
		fmt.Sprintf("%s fix %s --dry-run --diff --verbose --format json %s 2>/dev/null", dp.config.Path, relativeFilePath, configArg),
	)

	if result.Err != nil {
		log.Printf("Error running php-cs-fixer: %v", result.Err)
		return []protocol.Diagnostic{}, nil
	}

	var fullAnalysisResult PhpCsFixerOutputResult
	if err := json.Unmarshal(result.Stdout, &fullAnalysisResult); err != nil {
		log.Printf("Unmarshall err: %s", err)
		return []protocol.Diagnostic{}, nil
	}

	for _, file := range fullAnalysisResult.Files {
		for _, rule := range file.Rules {
			ruleResult := container.RunCommandInContainer(
				context.Background(),
				dp.config.Container,
				fmt.Sprintf("%s fix %s --dry-run --diff --verbose --format json --rules %s 2>/dev/null", dp.config.Path, relativeFilePath, rule),
			)

			if ruleResult.Err != nil {
				log.Printf("Error running php-cs-fixer for rule %s: %v", rule, ruleResult.Err)
				continue
			}

			var ruleAnalysisResult PhpCsFixerOutputResult
			if err := json.Unmarshal(ruleResult.Stdout, &ruleAnalysisResult); err != nil {
				log.Printf("Unmarshall err: %s", err)
				return []protocol.Diagnostic{}, nil
			}

			for _, file := range ruleAnalysisResult.Files {
				if file.Diff != "" {
					linesRange = dp.parseDiffForDiagnostics(file.Diff)
					for _, lineRange := range linesRange {
						diagnostics = append(diagnostics, protocol.Diagnostic{
							Range:    lineRange,
							Severity: protocol.DiagnosticSeverityWarning,
							Source:   dp.Name(),
							Message:  dp.explainRule(rule),
							Code:     rule,
						})
					}
				} else {
					log.Printf("No diff for file %s", file)
				}
			}
		}
	}

	return diagnostics, nil
}

func NewPhpCsFixer(providerConfig config.DiagnosticsProvider) *PhpCsFixer {
	return &PhpCsFixer{
		config: providerConfig,
	}
}

func (dp *PhpCsFixer) parseDiffForDiagnostics(diff string) []protocol.Range {
	var linesRange []protocol.Range

	lines := strings.Split(diff, "\n")
	originalLineNum, originalColNum, lineChange := 0, 0, false

	re := `@@\s+-(\d+),(\d+)?\s+\+(\d+),(\d+)?\s+@@`
	reC := regexp.MustCompile(re)

	for _, line := range lines {
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			matches := reC.FindStringSubmatch(line)
			if len(matches) >= 3 {
				if origLine, err := strconv.Atoi(matches[1]); err == nil {
					originalLineNum = origLine - 1
				}
				if origCol, err := strconv.Atoi(matches[2]); err == nil {
					originalColNum = origCol - 1
				}
			}
			continue
		}

		if len(line) == 0 {
			continue
		}

		switch line[0] {
		case '-':
			originalCode := strings.TrimPrefix(line, "-")
			linesRange = append(linesRange, protocol.Range{
				Start: protocol.Position{Line: uint32(originalLineNum), Character: uint32(originalColNum)},
				End:   protocol.Position{Line: uint32(originalLineNum), Character: uint32(len(strings.TrimSpace(originalCode)))},
			})
			lineChange = true
			originalLineNum++
		case '+':
			// If the line is changed, mark the removed lines, as that's where the error is.
			// If a new line is added (ex: "blank_line_before_statement") we need the added line
			if !lineChange {
				linesRange = append(linesRange, protocol.Range{
					Start: protocol.Position{Line: uint32(originalLineNum), Character: uint32(originalColNum)},
					End:   protocol.Position{Line: uint32(originalLineNum), Character: uint32(originalColNum)},
				})
			}
			originalLineNum++
		case ' ':
			originalLineNum++
		}
	}

	return linesRange
}

func (dp *PhpCsFixer) explainRule(rule string) string {
	if cachedDescription, ok := dp.ruleDescriptions.Load(rule); ok {
		return cachedDescription.(string)
	}

	result := container.RunCommandInContainer(
		context.Background(),
		dp.config.Container,
		fmt.Sprintf("%s describe %s 2>/dev/null", dp.config.Path, rule),
	)

	fullRuleDescription := strings.TrimSpace(string(result.Stdout))

	re1 := regexp.MustCompile(`Description of .* rule.`)
	ruleDescription := re1.ReplaceAllString(fullRuleDescription, "")
	re2 := regexp.MustCompile(`(?s)(Fixer is configurable|Fixer applying).*`)
	ruleDescription = re2.ReplaceAllString(ruleDescription, "")
	re3 := regexp.MustCompile(`(?s)Fixing examples:.*`)
	ruleDescription = re3.ReplaceAllString(ruleDescription, "")

	dp.ruleDescriptions.Store(rule, ruleDescription)

	return ruleDescription
}

// CanFormat returns true if formatting is enabled for this provider
func (dp *PhpCsFixer) CanFormat() bool {
	return dp.config.Format.Enabled
}

func (dp *PhpCsFixer) Format(ctx context.Context, filePath string, content string) (string, error) {
	if !dp.CanFormat() {
		return content, fmt.Errorf("formatting is not enabled for %s", dp.Name())
	}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		timeout := 30 * time.Second
		if dp.config.Format.TimeoutSeconds > 0 {
			timeout = time.Duration(dp.config.Format.TimeoutSeconds) * time.Second
		}
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
		log.Printf("%s%s Added %v timeout for php-cs-fixer formatting", logging.LogTagLSP, logging.LogTagServer, timeout)
	}

	configArg := ""
	if dp.config.ConfigFile != "" {
		configArg = fmt.Sprintf("--config %s", dp.config.ConfigFile)
	}

	cmd := fmt.Sprintf("%s fix - --diff %s", dp.config.Path, configArg)

	startTime := time.Now()
	result := container.RunCommandInContainer(ctx, dp.config.Container, cmd, content)
	duration := time.Since(startTime)

	if result.Err != nil {
		if ctx.Err() != nil {
			log.Printf("%s%s php-cs-fixer execution cancelled: %v", logging.LogTagLSP, logging.LogTagServer, ctx.Err())
			return content, fmt.Errorf("formatting cancelled: %w", ctx.Err())
		}

		log.Printf("%s%s php-cs-fixer failed after %v: %v", logging.LogTagLSP, logging.LogTagServer, duration, result.Err)
		return content, fmt.Errorf("php-cs-fixer command failed: %w", result.Err)
	}

	if result.ExitCode == 8 {
		log.Printf("%s%s php-cs-fixer found formatting changes (exit code 8) in %v", logging.LogTagLSP, logging.LogTagServer, duration)
	} else if result.ExitCode != 0 {
		log.Printf("%s%s php-cs-fixer returned non-zero exit code %d after %v", logging.LogTagLSP, logging.LogTagServer, result.ExitCode, duration)
		log.Printf("%s%s php-cs-fixer stderr: %s", logging.LogTagLSP, logging.LogTagServer, string(result.Stderr))
		return content, fmt.Errorf("php-cs-fixer failed with exit code %d", result.ExitCode)
	} else {
		log.Printf("%s%s php-cs-fixer completed successfully in %v, output length: %d bytes", logging.LogTagLSP, logging.LogTagServer, duration, len(result.Stdout))
	}

	diffStr := strings.TrimSpace(string(result.Stdout))
	if diffStr == "" {
		return content, nil
	}

	formattedContent, err := utils.ApplyUnifiedDiff(content, diffStr)
	if err != nil {
		return content, fmt.Errorf("failed to apply diff: %w", err)
	}

	return formattedContent, nil
}
