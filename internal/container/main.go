package container

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

func RunCommandInContainer(containerName string, containerCmd string, stdin ...string) ([]byte, error) {
	log.Printf("Running cmd: %s", containerCmd)

	stdinInput := ""
	if len(stdin) > 0 && stdin[0] != "" {
		stdinInput = stdin[0]
	}

	var cmd *exec.Cmd
	if stdinInput != "" {
		log.Printf("Using stdin input")
		cmd = exec.Command("docker", "exec", "-i", containerName, "sh", "-c", containerCmd)
		cmd.Stdin = strings.NewReader(stdinInput)
	} else {
		cmd = exec.Command("docker", "exec", containerName, "sh", "-c", containerCmd)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.Bytes(), fmt.Errorf("cmd returned error %s: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

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

func ValidateBinaryInContainer(containerName string, binaryPath string) error {
	containerCmd := fmt.Sprintf("which %s", binaryPath)
	cmdOutput, _ := RunCommandInContainer(containerName, containerCmd)

	if strings.TrimSpace(string(cmdOutput)) != binaryPath {
		return fmt.Errorf("binary %s not found in container %s; docker output: %s", binaryPath, containerName, cmdOutput)
	}

	return nil
}
