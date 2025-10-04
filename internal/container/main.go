package container

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// Making these vars allows for mocking in tests
var (
	RunCommandInContainer = func(containerName string, containerCmd string) ([]byte, error) {
		log.Printf("Running cmd: %s", containerCmd)
		cmd := exec.Command("docker", "exec", containerName, "sh", "-c", containerCmd)
		cmdOutput, err := cmd.Output()
		if err != nil {
			return cmdOutput, fmt.Errorf("cmd returned error %s", err)
		}

		return cmdOutput, nil
	}

	ValidateContainer = func(containerName string) error {
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

	ValidateBinaryInContainer = func(containerName string, binaryPath string) error {
		containerCmd := fmt.Sprintf("which %s", binaryPath)
		cmdOutput, _ := RunCommandInContainer(containerName, containerCmd)

		if strings.TrimSpace(string(cmdOutput)) != binaryPath {
			return fmt.Errorf("binary %s not found in container %s; docker output: %s", binaryPath, containerName, cmdOutput)
		}

		return nil
	}
)