package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/container"
	"github.com/cristianradulescu/php-diagls/internal/utils"
	"go.lsp.dev/protocol"
)

const (
	MagoProviderId   string = "mago"
	MagoProviderName string = "mago"

	MagoCommandLint    string = "lint"
	MagoCommandAnalyze string = "analyze"
)

// magoDefaultCommands are the mago subcommands run for diagnostics when the
// provider config doesn't list any.
var magoDefaultCommands = []string{MagoCommandLint, MagoCommandAnalyze}

// MagoReport is the shape of `mago lint|analyze --reporting-format json`.
type MagoReport struct {
	Issues []MagoIssue `json:"issues"`
}

type MagoIssue struct {
	Level       string           `json:"level"`
	Code        string           `json:"code"`
	Message     string           `json:"message"`
	Help        string           `json:"help"`
	Annotations []MagoAnnotation `json:"annotations"`
}

type MagoAnnotation struct {
	Message string   `json:"message"`
	Kind    string   `json:"kind"`
	Span    MagoSpan `json:"span"`
}

type MagoSpan struct {
	FileId struct {
		Name string `json:"name"`
	} `json:"file_id"`
	Start MagoPosition `json:"start"`
	End   MagoPosition `json:"end"`
}

// MagoPosition is a byte offset into the file plus its 0-based line.
type MagoPosition struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
}

type Mago struct {
	config   config.DiagnosticsProvider
	commands []string
}

func (dp *Mago) Id() string {
	return MagoProviderId
}

func (dp *Mago) Name() string {
	return MagoProviderName
}

// Analyze runs each configured mago command (lint and/or analyze) against
// the file concurrently and merges their diagnostics. A failing command
// doesn't discard the other's diagnostics; its error is returned alongside.
func (dp *Mago) Analyze(ctx context.Context, filePath string) ([]protocol.Diagnostic, error) {
	projectRoot := utils.FindProjectRoot(filePath)
	relativeFilePath, _ := filepath.Rel(projectRoot, filePath)

	if utils.IsPathExcluded(relativeFilePath, dp.config.ExcludePaths) {
		return nil, nil
	}

	// mago reports byte offsets; the file on disk is what it analyzed, so
	// read it once to turn those into LSP line/character positions.
	content, _ := os.ReadFile(filePath)

	results := make([][]protocol.Diagnostic, len(dp.commands))
	errs := make([]error, len(dp.commands))
	var wg sync.WaitGroup
	for i, command := range dp.commands {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = dp.runCommand(ctx, command, relativeFilePath, content)
		}()
	}
	wg.Wait()

	return mergeMagoDiagnostics(results), errors.Join(errs...)
}

// mergeMagoDiagnostics concatenates per-command diagnostics, dropping any
// that an earlier command already reported at the same place with the same
// code and message: lint and analyze both report every parse error.
func mergeMagoDiagnostics(results [][]protocol.Diagnostic) []protocol.Diagnostic {
	type key struct {
		rng     protocol.Range
		code    any
		message string
	}
	var merged []protocol.Diagnostic
	seen := map[key]bool{}
	for _, r := range results {
		for _, d := range r {
			k := key{d.Range, d.Code, d.Message}
			if seen[k] {
				continue
			}
			seen[k] = true
			merged = append(merged, d)
		}
	}
	return merged
}

func (dp *Mago) runCommand(ctx context.Context, command string, relativeFilePath string, content []byte) ([]protocol.Diagnostic, error) {
	result := container.RunCommandInContainer(
		ctx,
		dp.config.Container,
		fmt.Sprintf("%s%s %s --reporting-format=json %s", utils.ShellQuote(dp.config.Path), dp.configArg(), command, utils.ShellQuote(relativeFilePath)),
	)

	if result.Err != nil {
		return []protocol.Diagnostic{}, fmt.Errorf("running mago %s: %w", command, result.Err)
	}

	// mago exits 1 when it reports errors but still prints the JSON report;
	// output that doesn't parse means it never got that far.
	var report MagoReport
	if err := json.Unmarshal(result.Stdout, &report); err != nil {
		return []protocol.Diagnostic{}, fmt.Errorf("mago %s produced no report (exit %d): %s", command, result.ExitCode, utils.SummarizeOutput(result.Stderr, result.Stdout))
	}

	return dp.diagnosticsFromReport(report, command, relativeFilePath, content), nil
}

// configArg returns mago's global --config option (which must precede the
// subcommand), or "" when no config file is set.
func (dp *Mago) configArg() string {
	if dp.config.ConfigFile == "" {
		return ""
	}
	return " --config=" + utils.ShellQuote(dp.config.ConfigFile)
}

// diagnosticsFromReport converts a parsed mago JSON report into LSP
// diagnostics, keeping only issues located in relativeFilePath (or with no
// location at all). content is the analyzed file, used to turn mago's byte
// offsets into UTF-16 columns; when it doesn't cover an offset the
// diagnostic falls back to spanning the reported line.
func (dp *Mago) diagnosticsFromReport(report MagoReport, command string, relativeFilePath string, content []byte) []protocol.Diagnostic {
	var diagnostics []protocol.Diagnostic
	lineStarts := lineStartOffsets(content)
	source := dp.Name() + " " + command

	for _, issue := range report.Issues {
		annotation := primaryAnnotation(issue.Annotations)

		var rng protocol.Range
		if annotation != nil {
			name := filepath.ToSlash(annotation.Span.FileId.Name)
			if name != "" && name != filepath.ToSlash(relativeFilePath) {
				continue
			}
			rng = protocol.Range{
				Start: magoPositionToLSP(annotation.Span.Start, content, lineStarts),
				End:   magoPositionToLSP(annotation.Span.End, content, lineStarts),
			}
			if !positionsResolved(annotation.Span.Start, annotation.Span.End, content, lineStarts) {
				rng.End = protocol.Position{Line: rng.Start.Line, Character: 100}
			}
		} else {
			rng = protocol.Range{End: protocol.Position{Character: 100}}
		}

		message := issue.Message
		if annotation != nil && annotation.Message != "" {
			message += "\n" + annotation.Message
		}
		if issue.Help != "" {
			message += "\nHelp: " + issue.Help
		}

		diagnostic := protocol.Diagnostic{
			Range:    rng,
			Severity: magoSeverity(issue.Level),
			Source:   source,
			Message:  message,
		}
		if issue.Code != "" {
			diagnostic.Code = issue.Code
		}
		diagnostics = append(diagnostics, diagnostic)
	}

	return diagnostics
}

// primaryAnnotation returns the annotation that locates an issue: the first
// "Primary" one, else the first of any kind, else nil.
func primaryAnnotation(annotations []MagoAnnotation) *MagoAnnotation {
	for i := range annotations {
		if annotations[i].Kind == "Primary" {
			return &annotations[i]
		}
	}
	if len(annotations) > 0 {
		return &annotations[0]
	}
	return nil
}

func magoSeverity(level string) protocol.DiagnosticSeverity {
	switch strings.ToLower(level) {
	case "error":
		return protocol.DiagnosticSeverityError
	case "warning":
		return protocol.DiagnosticSeverityWarning
	case "note":
		return protocol.DiagnosticSeverityInformation
	default: // "help"
		return protocol.DiagnosticSeverityHint
	}
}

// lineStartOffsets returns the byte offset at which each line of content
// starts.
func lineStartOffsets(content []byte) []int {
	starts := []int{0}
	for i, b := range content {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// offsetResolvable reports whether pos's byte offset lies on pos's line in
// content, i.e. whether a precise column can be computed from it.
func offsetResolvable(pos MagoPosition, content []byte, lineStarts []int) bool {
	if pos.Line < 0 || pos.Line >= len(lineStarts) || pos.Offset > len(content) {
		return false
	}
	lineEnd := len(content)
	if pos.Line+1 < len(lineStarts) {
		lineEnd = lineStarts[pos.Line+1]
	}
	return pos.Offset >= lineStarts[pos.Line] && pos.Offset <= lineEnd
}

func positionsResolved(start, end MagoPosition, content []byte, lineStarts []int) bool {
	return offsetResolvable(start, content, lineStarts) && offsetResolvable(end, content, lineStarts)
}

// magoPositionToLSP converts a mago position to an LSP position, counting
// the column in UTF-16 code units. An unresolvable offset maps to the start
// of the reported line.
func magoPositionToLSP(pos MagoPosition, content []byte, lineStarts []int) protocol.Position {
	line := max(pos.Line, 0)
	if !offsetResolvable(pos, content, lineStarts) {
		return protocol.Position{Line: uint32(line), Character: 0}
	}
	prefix := content[lineStarts[pos.Line]:pos.Offset]
	if !utf8.Valid(prefix) {
		return protocol.Position{Line: uint32(line), Character: uint32(len(prefix))}
	}
	return protocol.Position{Line: uint32(line), Character: utf16Length(string(prefix))}
}

// CanFormat returns true if formatting is enabled for this provider
func (dp *Mago) CanFormat() bool {
	return dp.config.Format.Enabled
}

// Format pipes content through `mago format --stdin-input`, which prints the
// formatted file to stdout. --stdin-filepath lets mago apply its own
// formatter/source excludes to the buffer.
func (dp *Mago) Format(ctx context.Context, filePath string, content string) (string, error) {
	if !dp.CanFormat() {
		return content, fmt.Errorf("formatting is not enabled for %s", dp.Name())
	}
	if content == "" {
		return content, nil
	}

	projectRoot := utils.FindProjectRoot(filePath)
	relativeFilePath, _ := filepath.Rel(projectRoot, filePath)
	if utils.IsPathExcluded(relativeFilePath, dp.config.ExcludePaths) {
		return content, nil
	}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		timeout := 30 * time.Second
		if dp.config.Format.TimeoutSeconds > 0 {
			timeout = time.Duration(dp.config.Format.TimeoutSeconds) * time.Second
		}
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := fmt.Sprintf("%s%s format --stdin-input --stdin-filepath=%s", utils.ShellQuote(dp.config.Path), dp.configArg(), utils.ShellQuote(relativeFilePath))
	result := container.RunCommandInContainer(ctx, dp.config.Container, cmd, content)

	if result.Err != nil {
		if ctx.Err() != nil {
			return content, fmt.Errorf("formatting cancelled: %w", ctx.Err())
		}
		return content, fmt.Errorf("mago format command failed: %w", result.Err)
	}
	// Unlike php-cs-fixer, mago exits 0 whether or not it changed anything;
	// any non-zero exit (e.g. 2 on a parse error) is a failure.
	if result.ExitCode != 0 {
		return content, fmt.Errorf("mago format failed (exit %d): %s", result.ExitCode, utils.SummarizeOutput(result.Stderr, result.Stdout))
	}

	return string(result.Stdout), nil
}

// validateMagoCommands checks that every configured command is one mago
// provides diagnostics for.
func validateMagoCommands(commands []string) error {
	for _, command := range commands {
		if command != MagoCommandLint && command != MagoCommandAnalyze {
			return fmt.Errorf("unsupported mago command %q (supported: %s, %s)", command, MagoCommandLint, MagoCommandAnalyze)
		}
	}
	return nil
}

func NewMago(providerConfig config.DiagnosticsProvider) *Mago {
	commands := providerConfig.Commands
	if len(commands) == 0 {
		commands = magoDefaultCommands
	}
	return &Mago{
		config:   providerConfig,
		commands: commands,
	}
}
