package diagnostics_test

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
	"github.com/stretchr/testify/assert"
)

// mockCommandRunner is a mock implementation of the container.CommandRunner interface for testing.
type mockCommandRunner struct {
	ExecuteFunc func(containerName string, containerCmd string, stdin io.Reader) ([]byte, error)
}

// Execute calls the mock's ExecuteFunc.
func (m *mockCommandRunner) Execute(containerName string, containerCmd string, stdin io.Reader) ([]byte, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(containerName, containerCmd, stdin)
	}
	return nil, errors.New("ExecuteFunc not implemented")
}

func TestPhpCsFixer_Format(t *testing.T) {
	providerConfig := config.DiagnosticsProvider{
		Path:      "/usr/bin/php-cs-fixer",
		Container: "test-container",
	}

	t.Run("successful formatting", func(t *testing.T) {
		mockRunner := &mockCommandRunner{
			ExecuteFunc: func(containerName string, containerCmd string, stdin io.Reader) ([]byte, error) {
				return []byte("<?php echo 'formatted';"), nil
			},
		}
		phpcsfixer := diagnostics.NewPhpCsFixer(providerConfig, mockRunner)

		tempFile, err := createTempFile("<?php echo 'unformatted';")
		assert.NoError(t, err)
		defer os.Remove(tempFile)

		formatted, err := phpcsfixer.Format(tempFile)
		assert.NoError(t, err)
		assert.Equal(t, "<?php echo 'formatted';", formatted)
	})

	t.Run("container command error", func(t *testing.T) {
		mockRunner := &mockCommandRunner{
			ExecuteFunc: func(containerName string, containerCmd string, stdin io.Reader) ([]byte, error) {
				return nil, errors.New("container error")
			},
		}
		phpcsfixer := diagnostics.NewPhpCsFixer(providerConfig, mockRunner)

		tempFile, err := createTempFile("<?php echo 'unformatted';")
		assert.NoError(t, err)
		defer os.Remove(tempFile)

		_, err = phpcsfixer.Format(tempFile)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "container error"))
	})

	t.Run("file not found", func(t *testing.T) {
		mockRunner := &mockCommandRunner{}
		phpcsfixer := diagnostics.NewPhpCsFixer(providerConfig, mockRunner)

		_, err := phpcsfixer.Format("/non/existent/file.php")
		assert.Error(t, err)
	})
}

func createTempFile(content string) (string, error) {
	file, err := os.CreateTemp("", "test-*.php")
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return "", err
	}

	return file.Name(), nil
}