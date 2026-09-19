package diagnostics_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
)

func TestMago_IdAndName(t *testing.T) {
	provider := diagnostics.NewMago(config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container",
		Path:      "/usr/local/bin/mago",
	})

	if provider.Id() != "mago" {
		t.Errorf("Expected ID 'mago', got '%s'", provider.Id())
	}
	if provider.Name() != "mago" {
		t.Errorf("Expected name 'mago', got '%s'", provider.Name())
	}
}

func TestMago_Analyze(t *testing.T) {
	tests := []struct {
		name         string
		commands     []string
		wantInErrors []string
	}{
		{
			name:         "default runs lint and analyze",
			wantInErrors: []string{"mago lint", "mago analyze"},
		},
		{
			name:         "lint only",
			commands:     []string{"lint"},
			wantInErrors: []string{"mago lint"},
		},
		{
			name:         "analyze only",
			commands:     []string{"analyze"},
			wantInErrors: []string{"mago analyze"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := diagnostics.NewMago(config.DiagnosticsProvider{
				Enabled:   true,
				Container: "test-container-that-does-not-exist",
				Path:      "/usr/local/bin/mago",
				Commands:  tt.commands,
			})

			diags, err := provider.Analyze(t.Context(), t.TempDir()+"/test.php")

			// The container doesn't exist, so every configured command must
			// fail and be reported, not silently yield a clean file.
			if err == nil {
				t.Fatal("Analyze should report an error when the container is unavailable")
			}
			for _, want := range tt.wantInErrors {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Expected error to mention %q, got: %v", want, err)
				}
			}
			for _, cmd := range []string{"mago lint", "mago analyze"} {
				if !slices.Contains(tt.wantInErrors, cmd) && strings.Contains(err.Error(), cmd) {
					t.Errorf("Did not expect %q to run, got: %v", cmd, err)
				}
			}
			if len(diags) != 0 {
				t.Errorf("Expected no diagnostics, got %d", len(diags))
			}
		})
	}
}

func TestMago_Analyze_ExcludedPath(t *testing.T) {
	provider := diagnostics.NewMago(config.DiagnosticsProvider{
		Enabled:      true,
		Container:    "test-container-that-does-not-exist",
		Path:         "/usr/local/bin/mago",
		ExcludePaths: []string{"*.php"},
	})

	diags, err := provider.Analyze(t.Context(), t.TempDir()+"/test.php")
	if err != nil {
		t.Errorf("Excluded file should not run mago, got error: %v", err)
	}
	if len(diags) != 0 {
		t.Errorf("Expected no diagnostics for excluded file, got %d", len(diags))
	}
}

func TestMago_Format_Disabled(t *testing.T) {
	provider := diagnostics.NewMago(config.DiagnosticsProvider{
		Enabled:   true,
		Container: "test-container",
		Path:      "/usr/local/bin/mago",
	})

	if provider.CanFormat() {
		t.Error("CanFormat should be false when format.enabled is not set")
	}
	content := "<?php echo 1;"
	result, err := provider.Format(t.Context(), "/tmp/test.php", content)
	if err == nil {
		t.Error("Format should fail when formatting is disabled")
	}
	if result != content {
		t.Errorf("Expected original content, got %q", result)
	}
}
