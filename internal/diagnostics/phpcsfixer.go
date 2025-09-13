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
	"github.com/cristianradulescu/php-diagls/internal/utils"
	"go.lsp.dev/protocol"
)

const (
	PhpCsFixerProviderId   string = "phpcsfixer"
	PhpCsFixerProviderName string = "php-cs-fixer"
)

type OutputResultFiles []struct {
	Name  string   `json:"name"`
	Diff  string   `json:"diff"`
	Rules []string `json:"appliedFixers"`
}

type OutputResult struct {
	Files OutputResultFiles `json:"files"`
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

	fullAnalysisCmdOutput, _ := container.RunCommandInContainer(
		dp.config.Container,
		fmt.Sprintf("%s fix %s --dry-run --diff --verbose --format json --config %s 2>/dev/null", dp.config.Path, relativeFilePath, dp.config.ConfigFile),
	)
	// TODO: process specific phpcsfixer exit codes
	log.Printf("Full analysis cmd result: %s", string(fullAnalysisCmdOutput))

	var fullAnalysisResult OutputResult
	if err := json.Unmarshal(fullAnalysisCmdOutput, &fullAnalysisResult); err != nil {
		log.Printf("Unmarshall err: %s", err)
		return []protocol.Diagnostic{}, nil
	}

	log.Printf("Full analysis cmd result: %s", string(fullAnalysisCmdOutput))

	for _, file := range fullAnalysisResult.Files {
		for _, rule := range file.Rules {
			ruleAnalysisCmdOutput, _ := container.RunCommandInContainer(
				dp.config.Container,
				fmt.Sprintf("%s fix %s --dry-run --diff --verbose --format json --rules %s 2>/dev/null", dp.config.Path, relativeFilePath, rule),
			)

			log.Printf("Rule analysis cmd result: %s", string(ruleAnalysisCmdOutput))

			var ruleAnalysisResult OutputResult
			if err := json.Unmarshal(ruleAnalysisCmdOutput, &ruleAnalysisResult); err != nil {
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
	originalLineNum, originalColNum, _, inHeader := 0, 0, 0, true

	re := `@@\s+-(\d+)(?:,\d+)?\s+\+(\d+)(?:,\d+)?\s+@@`
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
			inHeader = false
			continue
		}

		if inHeader || len(line) == 0 {
			continue
		}

		switch line[0] {
		case '-':
			originalCode := strings.TrimPrefix(line, "-")
			linesRange = append(linesRange, protocol.Range{
				Start: protocol.Position{Line: uint32(originalLineNum), Character: uint32(originalColNum)},
				End:   protocol.Position{Line: uint32(originalLineNum), Character: uint32(len(strings.TrimSpace(originalCode)))},
			})
			originalLineNum++
		case '+':
			// We only mark the removed lines, as that's where the error is.
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
	re1 := regexp.MustCompile(`Description of the .* rule.`)
	ruleDescription := re1.ReplaceAllString(fullRuleDescription, "")
	re := regexp.MustCompile(`(?s)Fixer is configurable.*`)
	ruleDescription = re.ReplaceAllString(ruleDescription, "")

	ruleDescription = fmt.Sprintf("%s%s", utils.SnakeCaseToHumanReadable(rule), ruleDescription)

	dp.ruleDescriptions.Store(rule, ruleDescription)

	return ruleDescription
}
