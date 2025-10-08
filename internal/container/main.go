package container

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
)

// CommandRunner defines the interface for running commands.
type CommandRunner interface {
	Execute(containerName string, containerCmd string, stdin io.Reader) ([]byte, error)
}

// DockerCommandRunner implements the CommandRunner interface using Docker.
type DockerCommandRunner struct{}

// NewDockerCommandRunner creates a new DockerCommandRunner.
func NewDockerCommandRunner() *DockerCommandRunner {
	return &DockerCommandRunner{}
}

// Execute runs a command in the specified Docker container.
func (r *DockerCommandRunner) Execute(containerName string, containerCmd string, stdin io.Reader) ([]byte, error) {
	log.Printf("Running cmd: %s", containerCmd)

	args := []string{"exec", "-i", containerName, "sh", "-c", containerCmd}
	cmd := exec.Command("docker", args...)

	if stdin != nil {
		cmd.Stdin = stdin
	}

	cmdOutput, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return cmdOutput, fmt.Errorf("cmd returned error %s: %s", err, string(exitErr.Stderr))
		}
		return cmdOutput, fmt.Errorf("cmd returned error %s", err)
	}

	return cmdOutput, nil
}

// ValidateContainer checks if a container with the given name is running.
func ValidateContainer(containerName string) error {
	cmd := exec.Command("docker", "ps", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.Names}}")
	cmdOutput, err := cmd.Output()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(cmdOutput)) != containerName {
		return fmt.Errorf("container %s is not running; docker output: %s", containerName, cmdOutput)
	}

	return nil
}

// ValidateBinaryInContainer checks if a binary exists in the specified container.
func ValidateBinaryInContainer(runner CommandRunner, containerName string, binaryPath string) error {
	containerCmd := fmt.Sprintf("which %s", binaryPath)
	cmdOutput, _ := runner.Execute(containerName, containerCmd, nil)

	if strings.TrimSpace(string(cmdOutput)) != binaryPath {
		return fmt.Errorf("binary %s not found in container %s; docker output: %s", binaryPath, containerName, cmdOutput)
	}

	return nil
}