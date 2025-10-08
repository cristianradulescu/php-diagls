package formatter

import (
	"os"
	"strings"

	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
	"go.lsp.dev/protocol"
)

type Formatter struct {
	provider diagnostics.FormattingProvider
}

func NewFormatter(provider diagnostics.FormattingProvider) *Formatter {
	return &Formatter{
		provider: provider,
	}
}

func (f *Formatter) Format(filePath string) ([]protocol.TextEdit, error) {
	formattedContent, err := f.provider.Format(filePath)
	if err != nil {
		return nil, err
	}

	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if string(originalContent) == formattedContent {
		return []protocol.TextEdit{}, nil
	}

	return []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: uint32(len(strings.Split(string(originalContent), "\n"))), Character: 0},
			},
			NewText: formattedContent,
		},
	}, nil
}