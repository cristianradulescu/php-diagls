package diagnostics

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

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
	fullAnalysisCmdOutput, _ := container.RunCommandInContainer(
		dp.config.Container,
		fmt.Sprintf("%s fix %s --dry-run --diff --verbose --format json %s 2>/dev/null", dp.config.Path, relativeFilePath, configArg),
	)
	// TODO: process specific phpcsfixer exit codes
	// log.Printf("Full analysis cmd result: %s", string(fullAnalysisCmdOutput))

	var fullAnalysisResult PhpCsFixerOutputResult
	if err := json.Unmarshal(fullAnalysisCmdOutput, &fullAnalysisResult); err != nil {
		log.Printf("Unmarshall err: %s", err)
		return []protocol.Diagnostic{}, nil
	}

	// Run the analysis again, this time by specifying the rule. This should provide the better details for the diagnostics.
	for _, file := range fullAnalysisResult.Files {
		for _, rule := range file.Rules {
			ruleAnalysisCmdOutput, _ := container.RunCommandInContainer(
				dp.config.Container,
				fmt.Sprintf("%s fix %s --dry-run --diff --verbose --format json --rules %s 2>/dev/null", dp.config.Path, relativeFilePath, rule),
			)

			// log.Printf("Rule analysis cmd result: %s", string(ruleAnalysisCmdOutput))

			var ruleAnalysisResult PhpCsFixerOutputResult
			if err := json.Unmarshal(ruleAnalysisCmdOutput, &ruleAnalysisResult); err != nil {
				log.Printf("Unmarshall err: %s", err)
				return []protocol.Diagnostic{}, nil
			}

			// log.Printf("Rule analysis files: %v", ruleAnalysisResult.Files)

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

	fullRuleDescriptionOutput, _ := container.RunCommandInContainer(
		dp.config.Container,
		fmt.Sprintf("%s describe %s 2>/dev/null", dp.config.Path, rule),
	)

	fullRuleDescription := strings.TrimSpace(string(fullRuleDescriptionOutput))

	// Keep only the actual description without example and config info
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

func (dp *PhpCsFixer) Format(filePath string, content string) (string, error) {
	if !dp.CanFormat() {
		return content, fmt.Errorf("formatting is not enabled for %s", dp.Name())
	}

	// Build the php-cs-fixer command with --diff to get formatting changes
	configArg := ""
	if dp.config.ConfigFile != "" {
		configArg = fmt.Sprintf("--config %s", dp.config.ConfigFile)
	}

	// Use stdin with --diff to get the formatting diff without modifying files
	cmd := fmt.Sprintf("%s fix - --diff %s", dp.config.Path, configArg)

	diffOutput, err := container.RunCommandInContainer(dp.config.Container, cmd, content)
	if err != nil {
		log.Printf("%s%s php-cs-fixer returned non-zero exit code: %v", logging.LogTagLSP, logging.LogTagServer, err)

		// php-cs-fixer exit codes:
		// 0 = OK, no changes needed
		// 8 = Changes found (SUCCESS with --diff flag)
		// 1 = General errors
		// 2 = Configuration errors
		// 16 = Configuration file has invalid format
		// 32 = Configuration file has invalid content

		// Check if this is exit code 8 (changes found) which is success for --diff
		if strings.Contains(err.Error(), "exit status 8") {
			log.Printf("%s%s php-cs-fixer found formatting changes (exit code 8 is success)", logging.LogTagLSP, logging.LogTagServer)
			// Continue processing - this is actually success
		} else {
			log.Printf("%s%s php-cs-fixer failed with error: %v", logging.LogTagLSP, logging.LogTagServer, err)
			log.Printf("%s%s php-cs-fixer output (might contain error info): %s", logging.LogTagLSP, logging.LogTagServer, string(diffOutput))
			return content, fmt.Errorf("php-cs-fixer command failed (exit code indicates error): %w", err)
		}
	}

	// If no diff output, content is already formatted
	diffStr := strings.TrimSpace(string(diffOutput))
	if diffStr == "" {
		return content, nil
	}

	// Parse the diff and apply changes to get formatted content
	formattedContent, err := utils.ApplyUnifiedDiff(content, diffStr)
	if err != nil {
		return content, fmt.Errorf("failed to apply diff: %w", err)
	}

	return formattedContent, nil
}
