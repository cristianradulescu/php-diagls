package container

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestBoundedShellCommand_NoDeadlinePassthrough(t *testing.T) {
	if got := boundedShellCommand(context.Background(), "echo hi"); got != "echo hi" {
		t.Errorf("expected passthrough, got %q", got)
	}
}

func TestBoundedShellCommand_WrapsWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got := boundedShellCommand(ctx, "echo 'it''s' && exit 3")
	if !strings.Contains(got, "timeout 10 sh -c") {
		t.Errorf("expected a 10s timeout wrapper, got %q", got)
	}

	// The wrapper must preserve the inner command's quoting and exit code.
	out, err := exec.Command("sh", "-c", got).CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 3 {
		t.Fatalf("expected exit code 3, got err=%v out=%q", err, out)
	}
	if strings.TrimSpace(string(out)) != "its" {
		t.Errorf("unexpected output %q", out)
	}
}

func TestBoundedShellCommand_KillsAfterDeadline(t *testing.T) {
	if _, err := exec.LookPath("timeout"); err != nil {
		t.Skip("timeout binary not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err := exec.Command("sh", "-c", boundedShellCommand(ctx, "sleep 5")).Run()
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 124 {
		t.Fatalf("expected timeout exit code 124, got %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("command was not killed by the in-shell timeout")
	}
}
