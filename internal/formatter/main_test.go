package formatter_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/formatter"
	"github.com/stretchr/testify/assert"
	"go.lsp.dev/protocol"
)

type mockFormattingProvider struct {
	formatFunc func(filePath string) (string, error)
}

func (m *mockFormattingProvider) Format(filePath string) (string, error) {
	return m.formatFunc(filePath)
}

func TestFormatter_Format(t *testing.T) {
	tests := []struct {
		name            string
		provider        *mockFormattingProvider
		originalContent string
		expectedEdits   []protocol.TextEdit
		expectedErr     error
	}{
		{
			name: "successful formatting",
			provider: &mockFormattingProvider{
				formatFunc: func(filePath string) (string, error) {
					return "<?php echo 'Hello, world!';", nil
				},
			},
			originalContent: "<?php echo 'Hello, world!' ;",
			expectedEdits: []protocol.TextEdit{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 0},
						End:   protocol.Position{Line: uint32(len(strings.Split("<?php echo 'Hello, world!' ;", "\n"))), Character: 0},
					},
					NewText: "<?php echo 'Hello, world!';",
				},
			},
			expectedErr: nil,
		},
		{
			name: "formatting provider error",
			provider: &mockFormattingProvider{
				formatFunc: func(filePath string) (string, error) {
					return "", errors.New("provider error")
				},
			},
			originalContent: "<?php echo 'Hello, world!' ;",
			expectedEdits:   nil,
			expectedErr:     errors.New("provider error"),
		},
		{
			name: "no changes",
			provider: &mockFormattingProvider{
				formatFunc: func(filePath string) (string, error) {
					return "<?php echo 'Hello, world!';", nil
				},
			},
			originalContent: "<?php echo 'Hello, world!';",
			expectedEdits:   []protocol.TextEdit{},
			expectedErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile, err := createTempFile(tt.originalContent)
			assert.NoError(t, err)
			defer os.Remove(tempFile)

			f := formatter.NewFormatter(tt.provider)
			edits, err := f.Format(tempFile)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedEdits, edits)
			}
		})
	}
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