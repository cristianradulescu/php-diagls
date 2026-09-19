package diagnostics

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/container"
	"go.lsp.dev/protocol"
)

func TestPhpStan_diagnosticsFromReport(t *testing.T) {
	dp := NewPhpStan(config.DiagnosticsProvider{})

	// Real phpstan --error-format=json shape.
	raw := `{
	  "totals": {"errors": 0, "file_errors": 2},
	  "files": {
	    "/app/src/Foo.php": {
	      "errors": 2,
	      "messages": [
	        {"message": "Method Foo::bar() has no return type specified.", "line": 12, "ignorable": true, "identifier": "missingType.return"},
	        {"message": "Undefined variable: $x", "line": 0, "ignorable": false}
	      ]
	    }
	  },
	  "errors": []
	}`
	var report PhpstanOutputResult
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatal(err)
	}

	diags, err := dp.diagnosticsFromReport(report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(diags) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(diags))
	}

	byMsg := map[string]protocol.Diagnostic{}
	for _, d := range diags {
		byMsg[d.Message] = d
	}
	warn := byMsg["Method Foo::bar() has no return type specified."]
	if warn.Range.Start.Line != 11 || warn.Severity != protocol.DiagnosticSeverityWarning || warn.Code != "missingType.return" || warn.Source != "phpstan" {
		t.Errorf("unexpected ignorable diagnostic: %+v", warn)
	}
	fatal := byMsg["Undefined variable: $x"]
	if fatal.Range.Start.Line != 0 || fatal.Severity != protocol.DiagnosticSeverityError || fatal.Code != nil {
		t.Errorf("unexpected non-ignorable diagnostic: %+v", fatal)
	}
}

func TestPhpStan_diagnosticsFromReport_SurfacesTopLevelErrors(t *testing.T) {
	dp := NewPhpStan(config.DiagnosticsProvider{})
	report := PhpstanOutputResult{Errors: []string{"Config file phpstan.neon does not exist.", "second"}}

	diags, err := dp.diagnosticsFromReport(report)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics, got %d", len(diags))
	}
	if err == nil || !strings.Contains(err.Error(), "phpstan.neon does not exist") || !strings.Contains(err.Error(), "second") {
		t.Errorf("expected both errors surfaced, got %v", err)
	}
}

func TestPhpLint_diagnosticsFromOutput(t *testing.T) {
	dp := NewPhpLint(config.DiagnosticsProvider{})
	ok := &container.CommandResult{ExitCode: 0}
	failed := &container.CommandResult{ExitCode: 255}

	tests := []struct {
		name     string
		output   string
		result   *container.CommandResult
		wantLine uint32
		wantMsg  string
		wantErr  string
	}{
		{
			name:   "clean file",
			output: "No syntax errors detected in src/Foo.php\n",
			result: ok,
		},
		{
			name:   "clean file with deprecation notice first",
			output: "Deprecated: Optional parameter declared before required in src/Foo.php on line 3\nNo syntax errors detected in src/Foo.php\n",
			result: ok,
		},
		{
			name:     "parse error on stdout",
			output:   "PHP Parse error:  syntax error, unexpected token \"}\" in src/Foo.php on line 7\nErrors parsing src/Foo.php\n",
			result:   failed,
			wantLine: 6,
			wantMsg:  "syntax error, unexpected token \"}\"",
		},
		{
			name:     "fatal error without PHP prefix",
			output:   "Fatal error: Cannot redeclare foo() in /app/src/Foo.php on line 1\n",
			result:   failed,
			wantLine: 0,
			wantMsg:  "Cannot redeclare foo()",
		},
		{
			name:    "unrecognised failure is an error",
			output:  "Error response from daemon: No such container: php\n",
			result:  &container.CommandResult{ExitCode: 1},
			wantErr: "php -l failed (exit 1)",
		},
		{
			name:    "command error is propagated",
			output:  "",
			result:  &container.CommandResult{ExitCode: -1, Err: errors.New("docker missing")},
			wantErr: "docker missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags, err := dp.diagnosticsFromOutput(tt.output, tt.result)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantMsg == "" {
				if len(diags) != 0 {
					t.Errorf("expected no diagnostics, got %+v", diags)
				}
				return
			}
			if len(diags) != 1 {
				t.Fatalf("expected 1 diagnostic, got %d", len(diags))
			}
			d := diags[0]
			if d.Range.Start.Line != tt.wantLine || d.Message != tt.wantMsg || d.Severity != protocol.DiagnosticSeverityError || d.Source != "php-lint" {
				t.Errorf("unexpected diagnostic: %+v", d)
			}
		})
	}
}

func TestMago_diagnosticsFromReport(t *testing.T) {
	dp := NewMago(config.DiagnosticsProvider{})

	content := []byte("<?php\n\ndeclare(strict_types=1);\n\n$s = \"é😀\"; if ($s == \"x\") { echo $s; }\n")

	// Real `mago lint --reporting-format json` output for content (edits
	// and notes trimmed), plus issues exercising the fallbacks.
	raw := `{"issues": [
	  {"level": "Warning", "code": "identity-comparison", "message": "Use identity comparison ` + "`===`" + `.", "help": "Use ` + "`===`" + `.",
	   "annotations": [
	     {"message": "Other location", "kind": "Secondary", "span": {"file_id": {"name": "src/Utf.php"}, "start": {"offset": 0, "line": 0}, "end": {"offset": 5, "line": 0}}},
	     {"message": "Equality operator is used here", "kind": "Primary", "span": {"file_id": {"name": "src/Utf.php"}, "start": {"offset": 55, "line": 4}, "end": {"offset": 57, "line": 4}}}
	   ]},
	  {"level": "Error", "code": "parse", "message": "Parse error", "annotations": [
	     {"kind": "Primary", "span": {"file_id": {"name": "src/Utf.php"}, "start": {"offset": 9999, "line": 2}, "end": {"offset": 10000, "line": 2}}}
	   ]},
	  {"level": "Note", "code": "elsewhere", "message": "In another file", "annotations": [
	     {"kind": "Primary", "span": {"file_id": {"name": "src/Other.php"}, "start": {"offset": 0, "line": 0}, "end": {"offset": 1, "line": 0}}}
	   ]},
	  {"level": "Help", "message": "No location", "annotations": []}
	]}`
	var report MagoReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatal(err)
	}

	diags := dp.diagnosticsFromReport(report, "lint", "src/Utf.php", content)
	if len(diags) != 3 {
		t.Fatalf("expected 3 diagnostics (other-file issue dropped), got %d: %+v", len(diags), diags)
	}

	// Byte offset 55 is 22 bytes into line 4, after "é" (2 bytes, 1 UTF-16
	// unit) and "😀" (4 bytes, 2 UTF-16 units): column 19.
	cmp := diags[0]
	wantRange := protocol.Range{Start: protocol.Position{Line: 4, Character: 19}, End: protocol.Position{Line: 4, Character: 21}}
	if cmp.Range != wantRange {
		t.Errorf("expected range %+v, got %+v", wantRange, cmp.Range)
	}
	if cmp.Severity != protocol.DiagnosticSeverityWarning || cmp.Code != "identity-comparison" || cmp.Source != "mago lint" {
		t.Errorf("unexpected diagnostic: %+v", cmp)
	}
	if cmp.Message != "Use identity comparison `===`.\nEquality operator is used here\nHelp: Use `===`." {
		t.Errorf("unexpected message: %q", cmp.Message)
	}

	// Offsets outside the file fall back to spanning the reported line.
	parse := diags[1]
	if parse.Range != (protocol.Range{Start: protocol.Position{Line: 2}, End: protocol.Position{Line: 2, Character: 100}}) {
		t.Errorf("unexpected fallback range: %+v", parse.Range)
	}
	if parse.Severity != protocol.DiagnosticSeverityError || parse.Message != "Parse error" {
		t.Errorf("unexpected parse diagnostic: %+v", parse)
	}

	noLocation := diags[2]
	if noLocation.Range.Start.Line != 0 || noLocation.Severity != protocol.DiagnosticSeverityHint || noLocation.Code != nil {
		t.Errorf("unexpected location-less diagnostic: %+v", noLocation)
	}
}

func TestMago_diagnosticsFromReport_WithoutContent(t *testing.T) {
	dp := NewMago(config.DiagnosticsProvider{})
	report := MagoReport{Issues: []MagoIssue{{
		Level:   "Error",
		Code:    "invalid-argument",
		Message: "Invalid argument",
		Annotations: []MagoAnnotation{{
			Kind: "Primary",
			Span: MagoSpan{Start: MagoPosition{Offset: 40, Line: 3}, End: MagoPosition{Offset: 45, Line: 3}},
		}},
	}}}

	// An unreadable file must not lose the diagnostic, only its columns.
	diags := dp.diagnosticsFromReport(report, "analyze", "src/Foo.php", nil)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	want := protocol.Range{Start: protocol.Position{Line: 3}, End: protocol.Position{Line: 3, Character: 100}}
	if diags[0].Range != want || diags[0].Source != "mago analyze" {
		t.Errorf("unexpected diagnostic: %+v", diags[0])
	}
}

func TestMagoSeverity(t *testing.T) {
	tests := map[string]protocol.DiagnosticSeverity{
		"Error":   protocol.DiagnosticSeverityError,
		"Warning": protocol.DiagnosticSeverityWarning,
		"Note":    protocol.DiagnosticSeverityInformation,
		"Help":    protocol.DiagnosticSeverityHint,
	}
	for level, want := range tests {
		if got := magoSeverity(level); got != want {
			t.Errorf("magoSeverity(%q) = %v, want %v", level, got, want)
		}
	}
}

func TestValidateMagoCommands(t *testing.T) {
	if err := validateMagoCommands(nil); err != nil {
		t.Errorf("empty commands should be valid, got %v", err)
	}
	if err := validateMagoCommands([]string{"lint", "analyze"}); err != nil {
		t.Errorf("lint+analyze should be valid, got %v", err)
	}
	if err := validateMagoCommands([]string{"lint", "format"}); err == nil || !strings.Contains(err.Error(), `"format"`) {
		t.Errorf("expected unsupported command error, got %v", err)
	}
}

func TestMergeMagoDiagnostics(t *testing.T) {
	rng := protocol.Range{Start: protocol.Position{Line: 1, Character: 12}, End: protocol.Position{Line: 1, Character: 13}}
	parse := func(source string) protocol.Diagnostic {
		return protocol.Diagnostic{Range: rng, Code: "parse", Source: source, Message: "Parse error"}
	}
	other := protocol.Diagnostic{Range: rng, Code: "mixed-operand", Source: "mago analyze", Message: "Mixed"}

	merged := mergeMagoDiagnostics([][]protocol.Diagnostic{
		{parse("mago lint")},
		{parse("mago analyze"), other},
	})

	if len(merged) != 2 {
		t.Fatalf("expected the duplicate parse error to be dropped, got %+v", merged)
	}
	if merged[0].Source != "mago lint" || merged[1].Code != "mixed-operand" {
		t.Errorf("unexpected merge result: %+v", merged)
	}
}
