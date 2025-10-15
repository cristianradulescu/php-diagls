package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/container"
	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
	"github.com/cristianradulescu/php-diagls/internal/formatting"
	"github.com/cristianradulescu/php-diagls/internal/logging"
	"github.com/cristianradulescu/php-diagls/internal/utils"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// Server represents the Language Server Protocol (LSP) server
type Server struct {
	conn         jsonrpc2.Conn
	serverConfig *config.Config
}

// New creates a new LSP server instance
func New(conn jsonrpc2.Conn) *Server {
	s := &Server{
		conn:         conn,
		serverConfig: &config.Config{},
	}

	return s
}

func (s *Server) Handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	log.Printf("%s%s Received request: %s", logging.LogTagLSP, logging.LogTagServer, req.Method())

	switch req.Method() {
	case protocol.MethodInitialize:
		return s.handleInitialize(ctx, reply, req)
	case protocol.MethodInitialized:
		return s.handleInitialized(ctx, reply, req)
	case protocol.MethodWorkspaceExecuteCommand:
		return s.handleExecuteCommand(ctx, reply, req)
	case protocol.MethodTextDocumentDidOpen:
		return s.handleDidOpen(ctx, reply, req)
	case protocol.MethodTextDocumentDidChange:
		return s.handleDidChange(ctx, reply, req)
	case protocol.MethodTextDocumentDidClose:
		return s.handleDidClose(ctx, reply, req)
	case protocol.MethodTextDocumentDidSave:
		return s.handleDidSave(ctx, reply, req)
	case protocol.MethodTextDocumentFormatting:
		return s.handleDocumentFormatting(ctx, reply, req)
	case protocol.MethodTextDocumentCodeAction:
		return s.handleCodeAction(ctx, reply, req)
	case protocol.MethodWorkspaceDidChangeWatchedFiles:
		return s.handleDidChangeWatchedFiles(ctx, reply, req)
	case protocol.MethodShutdown:
		return s.handleShutdown(ctx, reply, req)
	case protocol.MethodExit:
		return s.handleExit(ctx, reply, req)
	case protocol.MethodCancelRequest:
		return s.handleCancelRequest(ctx, reply, req)
	default:
		log.Printf("%s%s Unhandled method: %s", logging.LogTagLSP, logging.LogTagServer, req.Method())
		return reply(ctx, nil, nil)
	}
}

func (s *Server) handleInitialize(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	log.Printf("%s%s Handling initialize request", logging.LogTagLSP, logging.LogTagServer)

	var params protocol.InitializeParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling initialize params: %v", logging.LogTagLSP, logging.LogTagServer, err)

		return err
	}

	log.Printf("%s%s Client info: name=%s, version=%s", logging.LogTagLSP, logging.LogTagServer, params.ClientInfo.Name, params.ClientInfo.Version)

	// Load configuration. Show warning if not found and exit
	if !s.serverConfig.IsInitialized() {
		// Determine project root from workspace folder URI or RootURI
		projectRoot := ""
		if len(params.WorkspaceFolders) > 0 && params.WorkspaceFolders[0].URI != "" {
			projectRoot = utils.URIToPath(protocol.DocumentURI(params.WorkspaceFolders[0].URI))
		} else if params.RootURI != "" {
			projectRoot = utils.URIToPath(protocol.DocumentURI(params.RootURI))
		} else {
			if cwd, cwdErr := os.Getwd(); cwdErr == nil {
				projectRoot = cwd
			}
		}
		serverConfig, err := s.serverConfig.LoadConfig(projectRoot)
		if err != nil {
			log.Printf("%s%s No config: %v", logging.LogTagLSP, logging.LogTagServer, err)

			// s.showWindowMessage(ctx, protocol.MessageTypeWarning, fmt.Sprintf("%s", err))
			// s.showWindowMessage(ctx, protocol.MessageTypeWarning, "Exiting...")
			os.Exit(0)
		}
		s.serverConfig = serverConfig
	}

	resp := protocol.InitializeResult{
		Capabilities: serverCapabilities(),
		ServerInfo:   serverInfo(),
	}

	return reply(ctx, resp, nil)
}

func (s *Server) handleInitialized(ctx context.Context, reply jsonrpc2.Replier, _ jsonrpc2.Request) error {
	log.Printf("%s%s Client initialized successfully", logging.LogTagLSP, logging.LogTagServer)

	return reply(ctx, nil, nil)
}

func (s *Server) handleExecuteCommand(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.ExecuteCommandParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling executeCommand params: %v", logging.LogTagLSP, logging.LogTagServer, err)
		return err
	}

	log.Printf("%s%s Executing command: %s", logging.LogTagLSP, logging.LogTagServer, params.Command)

	switch params.Command {
	case getFullLspCommandName(LspCommandNameShowConfig):
		return s.handleShowConfigCommand(ctx, reply)

	default:
		return reply(ctx, nil, fmt.Errorf("unknown command: %s", params.Command))
	}
}

func (s *Server) handleShowConfigCommand(ctx context.Context, reply jsonrpc2.Replier) error {
	s.showWindowMessage(ctx, protocol.MessageTypeInfo, fmt.Sprintf("Current configuration: %s", s.serverConfig.RawData))

	return reply(ctx, nil, nil)
}

func (s *Server) handleDidOpen(ctx context.Context, _ jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling %s params: %v", logging.LogTagLSP, logging.LogTagServer, req.Method(), err)

		return err
	}

	diagnostics := s.collectDiagnostics(ctx, params.TextDocument.URI.Filename())
	s.publishDiagnostics(ctx, params.TextDocument.URI, diagnostics)

	return nil
}

func (s *Server) handleDidChange(ctx context.Context, _ jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling %s params: %v", logging.LogTagLSP, logging.LogTagServer, req.Method(), err)

		return err
	}

	diagnostics := s.collectDiagnostics(ctx, params.TextDocument.URI.Filename())
	s.publishDiagnostics(ctx, params.TextDocument.URI, diagnostics)

	return nil
}

func (s *Server) handleDidSave(ctx context.Context, _ jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling %s params: %v", logging.LogTagLSP, logging.LogTagServer, req.Method(), err)

		return err
	}

	diagnostics := s.collectDiagnostics(ctx, params.TextDocument.URI.Filename())
	s.publishDiagnostics(ctx, params.TextDocument.URI, diagnostics)

	return nil
}

func (s *Server) handleDidChangeWatchedFiles(ctx context.Context, _ jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeWatchedFilesParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling %s params: %v", logging.LogTagLSP, logging.LogTagServer, req.Method(), err)

		return err
	}

	for _, change := range params.Changes {
		if strings.HasSuffix(string(change.URI), ".php") {
			switch change.Type {
			case protocol.FileChangeTypeChanged, protocol.FileChangeTypeCreated:
				diagnostics := s.collectDiagnostics(ctx, change.URI.Filename())
				s.publishDiagnostics(ctx, change.URI, diagnostics)
			case protocol.FileChangeTypeDeleted:
				s.publishDiagnostics(ctx, change.URI, []protocol.Diagnostic{})
			}
		}
	}

	return nil
}

func (s *Server) handleDidClose(ctx context.Context, _ jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling %s params: %v", logging.LogTagLSP, logging.LogTagServer, req.Method(), err)

		return err
	}

	diagnostics := s.collectDiagnostics(ctx, params.TextDocument.URI.Filename())
	s.publishDiagnostics(ctx, params.TextDocument.URI, diagnostics)

	return nil
}

func (s *Server) handleShutdown(ctx context.Context, reply jsonrpc2.Replier, _ jsonrpc2.Request) error {
	log.Printf("%s%s Performing cleanup before shutdown", logging.LogTagLSP, logging.LogTagServer)

	return reply(ctx, nil, nil)
}

func (s *Server) handleExit(_ context.Context, _ jsonrpc2.Replier, _ jsonrpc2.Request) error {
	log.Printf("%s%s Exiting server", logging.LogTagLSP, logging.LogTagServer)

	return s.conn.Close()
}

func (s *Server) handleCancelRequest(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params struct {
		ID interface{} `json:"id"`
	}
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling cancel request params: %v", logging.LogTagLSP, logging.LogTagServer, err)
		return err
	}

	log.Printf("%s%s Client requested cancellation for request ID: %v", logging.LogTagLSP, logging.LogTagServer, params.ID)
	// Note: The actual cancellation is handled by the jsonrpc2 library's context cancellation mechanism
	// This handler acknowledges the cancel request - the running operation should detect ctx.Done()
	return reply(ctx, nil, nil)
}

func (s *Server) showWindowMessage(ctx context.Context, messageType protocol.MessageType, message string) {
	params := &protocol.ShowMessageParams{Type: messageType, Message: message}
	if err := s.conn.Notify(ctx, protocol.MethodWindowShowMessage, params); err != nil {
		log.Printf("%s%s Failed to send window message: %v", logging.LogTagLSP, logging.LogTagServer, err)
	}
}

func (s *Server) publishDiagnostics(ctx context.Context, uri protocol.DocumentURI, diagnostics []protocol.Diagnostic) {
	params := protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: utils.EnsureDiagnosticsArray(diagnostics),
	}

	if err := s.conn.Notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, params); err != nil {
		log.Printf("%s%s Failed to publish diagnostics: %v", logging.LogTagLSP, logging.LogTagServer, err)
	}
}

func (s *Server) loadDiagnosticsProviders() []diagnostics.DiagnosticsProvider {
	providers := []diagnostics.DiagnosticsProvider{}

	for id, providerConfig := range s.serverConfig.DiagnosticsProviders {
		// Initialize only enabled diagnostics providers
		if !providerConfig.Enabled {
			continue
		}

		provider, err := diagnostics.NewDiagnosticsProvider(id, providerConfig)
		if err != nil {
			s.showWindowMessage(context.Background(), protocol.MessageTypeError, fmt.Sprintf("%v", err))

			continue
		}

		providers = append(providers, provider)
	}

	return providers
}

func (s *Server) collectDiagnostics(ctx context.Context, filePath string) []protocol.Diagnostic {
	var diagnostics []protocol.Diagnostic

	ignoredDirs := []string{"/vendor/", "/var/cache/"}
	for _, dir := range ignoredDirs {
		if strings.Contains(filePath, dir) {
			return diagnostics
		}
	}

	providers := s.loadDiagnosticsProviders()
	if len(providers) == 0 {
		return diagnostics
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	wg.Add(len(providers))
	for _, provider := range providers {
		p := provider
		go func() {
			defer wg.Done()

			providerDiagnostics, err := p.Analyze(filePath)
			if err != nil {
				s.showWindowMessage(ctx, protocol.MessageTypeError, fmt.Sprintf("Diagnostics provider %s failed: %v", p.Name(), err))
				return
			}

			mu.Lock()
			diagnostics = append(diagnostics, providerDiagnostics...)
			mu.Unlock()
		}()
	}
	wg.Wait()

	return diagnostics
}

func (s *Server) loadFormattingProviders() []formatting.FormattingProvider {
	return formatting.LoadFormattingProviders(s.serverConfig.DiagnosticsProviders)
}

func (s *Server) handleCodeAction(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.CodeActionParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling code action params: %v", logging.LogTagLSP, logging.LogTagServer, err)
		return err
	}

	// Ensure php-cs-fixer provider is configured
	providerCfg, ok := s.getPhpCsFixerProviderConfig()
	if !ok {
		return reply(ctx, []protocol.CodeAction{}, nil)
	}

	fileURI := params.TextDocument.URI
	filePath := fileURI.Filename()

	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("%s%s Error reading file for code action: %v", logging.LogTagLSP, logging.LogTagServer, err)
		return reply(ctx, []protocol.CodeAction{}, nil)
	}
	original := string(content)

	var actions []protocol.CodeAction

	// Add a quick fix to apply all php-cs-fixer fixes for the file
	{
		configArg := ""
		if providerCfg.ConfigFile != "" {
			configArg = fmt.Sprintf("--config %s", providerCfg.ConfigFile)
		}
		cmd := fmt.Sprintf("%s fix - --diff %s", providerCfg.Path, configArg)
		diffOutput, allErr := container.RunCommandInContainer(ctx, providerCfg.Container, cmd, original)
		diffStr := strings.TrimSpace(string(diffOutput))

		// Treat exit status 8 (changes found) as success for diff mode
		if diffStr != "" && (allErr == nil || strings.Contains(allErr.Error(), "exit status 8")) {
			if formatted, applyErr := utils.ApplyUnifiedDiff(original, diffStr); applyErr == nil && formatted != original {
				lines := strings.Split(original, "\n")
				endLine := uint32(len(lines) - 1)
				endCharacter := uint32(0)
				if len(lines) > 0 {
					endCharacter = uint32(len(lines[len(lines)-1]))
				}

				fullEdits := []protocol.TextEdit{
					{
						Range: protocol.Range{
							Start: protocol.Position{Line: 0, Character: 0},
							End:   protocol.Position{Line: endLine, Character: endCharacter},
						},
						NewText: formatted,
					},
				}

				weAll := protocol.WorkspaceEdit{
					Changes: map[protocol.DocumentURI][]protocol.TextEdit{
						fileURI: fullEdits,
					},
				}

				actions = append(actions, protocol.CodeAction{
					Title:       "Apply php-cs-fixer: Fix all issues",
					Kind:        protocol.QuickFix,
					Edit:        &weAll,
					Diagnostics: []protocol.Diagnostic{},
				})
			}
		}
	}

	return reply(ctx, actions, nil)
}

func (s *Server) getPhpCsFixerProviderConfig() (config.DiagnosticsProvider, bool) {
	for id, cfg := range s.serverConfig.DiagnosticsProviders {
		if id == diagnostics.PhpCsFixerProviderId && cfg.Enabled {
			return cfg, true
		}
	}
	return config.DiagnosticsProvider{}, false
}

func (s *Server) handleDocumentFormatting(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DocumentFormattingParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling document formatting params: %v", logging.LogTagLSP, logging.LogTagServer, err)
		return err
	}

	filePath := params.TextDocument.URI.Filename()
	log.Printf("%s%s Formatting document: %s", logging.LogTagLSP, logging.LogTagServer, filePath)

	// Check if context already has a deadline
	if deadline, ok := ctx.Deadline(); ok {
		log.Printf("%s%s Format request has deadline: %v", logging.LogTagLSP, logging.LogTagServer, deadline)
	}

	// Read current file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("%s%s Error reading file for formatting: %v", logging.LogTagLSP, logging.LogTagServer, err)
		return reply(ctx, nil, fmt.Errorf("failed to read file: %w", err))
	}

	formattingProviders := s.loadFormattingProviders()
	if len(formattingProviders) == 0 {
		log.Printf("%s%s No formatting providers available", logging.LogTagLSP, logging.LogTagServer)
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	// Apply formatting using the first available provider
	// In the future, this could be configurable or try multiple providers
	provider := formattingProviders[0]
	log.Printf("%s%s Starting formatting with %s", logging.LogTagLSP, logging.LogTagServer, provider.Name())

	formattedContent, err := provider.Format(ctx, filePath, string(content))
	if err != nil {
		// Check if error is due to cancellation
		if ctx.Err() != nil {
			log.Printf("%s%s Formatting cancelled by client: %v", logging.LogTagLSP, logging.LogTagServer, ctx.Err())
			s.showWindowMessage(ctx, protocol.MessageTypeWarning, "Formatting operation was cancelled")
		} else {
			log.Printf("%s%s Formatting failed with %s: %v", logging.LogTagLSP, logging.LogTagServer, provider.Name(), err)
			s.showWindowMessage(ctx, protocol.MessageTypeError, fmt.Sprintf("Error formatting with %s: %v", provider.Name(), err))
		}
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	log.Printf("%s%s Formatting completed successfully with %s", logging.LogTagLSP, logging.LogTagServer, provider.Name())

	// If content hasn't changed, return empty edits
	if formattedContent == string(content) {
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	// Calculate the range for the entire document
	lines := strings.Split(string(content), "\n")
	endLine := uint32(len(lines) - 1)
	endCharacter := uint32(0)
	if len(lines) > 0 {
		endCharacter = uint32(len(lines[len(lines)-1]))
	}

	// Return a single text edit that replaces the entire document
	textEdits := []protocol.TextEdit{
		{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: endLine, Character: endCharacter},
			},
			NewText: formattedContent,
		},
	}

	return reply(ctx, textEdits, nil)
}
