package container

import (
	"fmt"
	"os/exec"
	"strings"
)

func ValidateContainer(containerName string) error {
	cmd := exec.Command("docker", "ps", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.Names}}")
	cmdOutput, err := cmd.Output()
	if err != nil {
		return err
	}

	if (strings.TrimSpace(string(cmdOutput)) != containerName) {
		return fmt.Errorf("container %s is not running; docker output: %s", containerName, cmdOutput)
	}

	return nil
}
 
func ValidateBinaryInContainer(containerName string, binaryPath string) error {
	containerCmd  := fmt.Sprintf("which %s", binaryPath)
	cmd := exec.Command("docker", "exec", containerName, "sh", "-c", containerCmd)
	cmdOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("cmd `%s` returned error %s", cmd, err)
	}

	if (strings.TrimSpace(string(cmdOutput)) != binaryPath) {
		return fmt.Errorf("binary %s not found in container %s; docker output: %s", binaryPath, containerName, cmdOutput)
	}

	return nil
}
