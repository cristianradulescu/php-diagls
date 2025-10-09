package server

import (
	"fmt"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"go.lsp.dev/protocol"
)

const (
	LspCommandPrefix = config.Name
	LspCommandSeparator = "/"
	LspCommandNameShowConfig = "showConfig"
)

func serverCapabilities() protocol.ServerCapabilities {
	return protocol.ServerCapabilities{
		TextDocumentSync: &protocol.TextDocumentSyncOptions{
			Change:    protocol.TextDocumentSyncKindFull,
			OpenClose: true,
			Save:      &protocol.SaveOptions{IncludeText: false},
		},
		ExecuteCommandProvider: &protocol.ExecuteCommandOptions{
			Commands: []string{
				getFullLspCommandName(LspCommandNameShowConfig),
			},
		},
		DocumentFormattingProvider: true,
	}
}

func serverInfo() *protocol.ServerInfo {
	return &protocol.ServerInfo{
		Name:    string(config.Name),
		Version: string(config.Version),
	}
}

func getFullLspCommandName(command string) string {
	return fmt.Sprintf("%s%s%s", LspCommandPrefix, LspCommandSeparator, command)
}
