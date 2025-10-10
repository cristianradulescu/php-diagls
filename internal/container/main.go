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

func RunCommandInContainer(ctx context.Context, containerName string, containerCmd string, stdin ...string) ([]byte, error) {
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

	// Start the command
	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Wait for completion or cancellation
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return stdout.Bytes(), fmt.Errorf("cmd returned error %s: %s", err, stderr.String())
		}
		return stdout.Bytes(), nil
	case <-ctx.Done():
		// Context cancelled - kill the process
		log.Printf("Command cancelled, killing process: %s", containerCmd)
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		// Wait for the process to finish after killing
		<-done
		return stdout.Bytes(), fmt.Errorf("command cancelled: %w", ctx.Err())
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
	cmdOutput, _ := RunCommandInContainer(ctx, containerName, containerCmd)

	if strings.TrimSpace(string(cmdOutput)) != binaryPath {
		return fmt.Errorf("binary %s not found in container %s; docker output: %s", binaryPath, containerName, cmdOutput)
	}

	return nil
}
