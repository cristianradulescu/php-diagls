package container

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"os/exec"
	"strings"
	"time"

	"github.com/cristianradulescu/php-diagls/internal/utils"
)

type CommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Err      error
}

// RunCommandInContainer runs containerCmd through `sh -c` inside containerName
// via docker exec. When ctx carries a deadline, the command is also wrapped in
// the container's own `timeout` (when available) so that the process inside
// the container stops too: killing the docker CLI client on the host does not
// terminate the exec'd process, it only detaches from it.
func RunCommandInContainer(ctx context.Context, containerName string, containerCmd string, stdin ...string) *CommandResult {
	log.Printf("Running cmd: %s", containerCmd)

	stdinInput := ""
	if len(stdin) > 0 && stdin[0] != "" {
		stdinInput = stdin[0]
	}

	shellCmd := boundedShellCommand(ctx, containerCmd)

	var cmd *exec.Cmd
	if stdinInput != "" {
		log.Printf("Using stdin input")
		cmd = exec.CommandContext(ctx, "docker", "exec", "-i", containerName, "sh", "-c", shellCmd)
		cmd.Stdin = strings.NewReader(stdinInput)
	} else {
		cmd = exec.CommandContext(ctx, "docker", "exec", containerName, "sh", "-c", shellCmd)
	}
	// If the docker client ignores SIGKILL's effect on its pipes (e.g. a
	// grandchild still holds stdout), don't block Wait forever.
	cmd.WaitDelay = 2 * time.Second

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return &CommandResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: 0}
	}

	if ctx.Err() != nil {
		log.Printf("Command cancelled: %s", containerCmd)
		return &CommandResult{
			Stdout:   stdout.Bytes(),
			Stderr:   stderr.Bytes(),
			ExitCode: -1,
			Err:      fmt.Errorf("command cancelled: %w", ctx.Err()),
		}
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return &CommandResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: exitErr.ExitCode()}
	}

	return &CommandResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: -1,
		Err:      fmt.Errorf("failed to start command: %w", err),
	}
}

// boundedShellCommand wraps containerCmd so it is killed inside the container
// once ctx's deadline passes. Images without a `timeout` binary fall back to
// running the command unbounded, which is no worse than before.
func boundedShellCommand(ctx context.Context, containerCmd string) string {
	deadline, ok := ctx.Deadline()
	if !ok {
		return containerCmd
	}

	seconds := int(math.Ceil(time.Until(deadline).Seconds()))
	if seconds < 1 {
		seconds = 1
	}

	quoted := utils.ShellQuote(containerCmd)
	return fmt.Sprintf(
		"if command -v timeout >/dev/null 2>&1; then exec timeout %d sh -c %s; else exec sh -c %s; fi",
		seconds, quoted, quoted,
	)
}

func ValidateContainer(containerName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "ps", "--filter", fmt.Sprintf("name=^%s$", containerName), "--format", "{{.Names}}")
	cmdOutput, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("container validation timed out for %s", containerName)
		}
		return err
	}

	if strings.TrimSpace(string(cmdOutput)) != containerName {
		return fmt.Errorf("container %s is not running; docker output: %s", containerName, cmdOutput)
	}

	return nil
}

// ValidateBinaryInContainer checks that binaryPath is an executable file in
// containerName. It uses `test -x` rather than `which`, which is not shipped
// by every base image.
func ValidateBinaryInContainer(containerName string, binaryPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	containerCmd := fmt.Sprintf("test -x %s", utils.ShellQuote(binaryPath))
	result := RunCommandInContainer(ctx, containerName, containerCmd)

	if result.Err != nil {
		return fmt.Errorf("could not check binary %s in container %s: %w", binaryPath, containerName, result.Err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("binary %s not found or not executable in container %s (exit %d): %s", binaryPath, containerName, result.ExitCode, utils.SummarizeOutput(result.Stderr, result.Stdout))
	}

	return nil
}
