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
