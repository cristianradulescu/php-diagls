package container

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

type CommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Err      error
}

func RunCommandInContainer(ctx context.Context, containerName string, containerCmd string, stdin ...string) *CommandResult {
	log.Printf("Running cmd: %s", containerCmd)

	stdinInput := ""
	if len(stdin) > 0 && stdin[0] != "" {
		stdinInput = stdin[0]
	}

	var cmd *exec.Cmd
	if stdinInput != "" {
		log.Printf("Using stdin input")
		cmd = exec.CommandContext(ctx, "docker", "exec", "-i", containerName, "sh", "-c", containerCmd)
		cmd.Stdin = strings.NewReader(stdinInput)
	} else {
		cmd = exec.CommandContext(ctx, "docker", "exec", containerName, "sh", "-c", containerCmd)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Start()
	if err != nil {
		return &CommandResult{
			Stdout:   nil,
			Stderr:   nil,
			ExitCode: -1,
			Err:      fmt.Errorf("failed to start command: %w", err),
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return &CommandResult{
					Stdout:   stdout.Bytes(),
					Stderr:   stderr.Bytes(),
					ExitCode: -1,
					Err:      err,
				}
			}
		}
		return &CommandResult{
			Stdout:   stdout.Bytes(),
			Stderr:   stderr.Bytes(),
			ExitCode: exitCode,
			Err:      nil,
		}
	case <-ctx.Done():
		log.Printf("Command cancelled, killing process: %s", containerCmd)
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		<-done
		return &CommandResult{
			Stdout:   stdout.Bytes(),
			Stderr:   stderr.Bytes(),
			ExitCode: -1,
			Err:      fmt.Errorf("command cancelled: %w", ctx.Err()),
		}
	}
}

func ValidateContainer(containerName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "ps", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.Names}}")
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

func ValidateBinaryInContainer(containerName string, binaryPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	containerCmd := fmt.Sprintf("which %s", binaryPath)
	result := RunCommandInContainer(ctx, containerName, containerCmd)

	if strings.TrimSpace(string(result.Stdout)) != binaryPath {
		return fmt.Errorf("binary %s not found in container %s; docker output: %s", binaryPath, containerName, result.Stdout)
	}

	return nil
}
