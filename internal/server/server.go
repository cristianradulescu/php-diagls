package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
	"github.com/cristianradulescu/php-diagls/internal/formatting"
	"github.com/cristianradulescu/php-diagls/internal/logging"
	"github.com/cristianradulescu/php-diagls/internal/utils"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

const (
	diagnosticsDebounceInterval = 300 * time.Millisecond
	formattingDebounceInterval  = 100 * time.Millisecond
)

// Server represents the Language Server Protocol (LSP) server
type Server struct {
	conn         jsonrpc2.Conn
	serverConfig *config.Config

	diagnosticsProviders []diagnostics.DiagnosticsProvider
	formattingProviders  []formatting.FormattingProvider

	// In-memory document cache for synchronized content
	docMu     sync.RWMutex
	documents map[protocol.DocumentURI]string

	// Debounce for diagnostics (per-file) with last-wins strategy
	diagMu     sync.Mutex
	diagTimers map[protocol.DocumentURI]*time.Timer
	diagGen    map[protocol.DocumentURI]uint64

	// Debounce for formatting (per-file) with last-wins strategy
	fmtMu     sync.Mutex
	fmtTimers map[protocol.DocumentURI]*time.Timer
	fmtGen    map[protocol.DocumentURI]uint64
}

// New creates a new LSP server instance
func New(conn jsonrpc2.Conn) *Server {
	s := &Server{
		conn:         conn,
		serverConfig: &config.Config{},
		documents:    make(map[protocol.DocumentURI]string),
		diagTimers:   make(map[protocol.DocumentURI]*time.Timer),
		diagGen:      make(map[protocol.DocumentURI]uint64),
		fmtTimers:    make(map[protocol.DocumentURI]*time.Timer),
		fmtGen:       make(map[protocol.DocumentURI]uint64),
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
			os.Exit(0)
		}
		s.serverConfig = serverConfig

		// Preload diagnostics and formatting providers once
		_ = s.loadDiagnosticsProviders()
		_ = s.loadFormattingProviders()
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

	s.setDocumentContent(params.TextDocument.URI, params.TextDocument.Text)
	s.scheduleDiagnostics(params.TextDocument.URI)

	return nil
}

func (s *Server) handleDidChange(ctx context.Context, _ jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling %s params: %v", logging.LogTagLSP, logging.LogTagServer, req.Method(), err)

		return err
	}

	if len(params.ContentChanges) > 0 {
		lastChange := params.ContentChanges[len(params.ContentChanges)-1]
		s.setDocumentContent(params.TextDocument.URI, lastChange.Text)
	}

	s.scheduleDiagnostics(params.TextDocument.URI)

	return nil
}

func (s *Server) handleDidSave(ctx context.Context, _ jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidSaveTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		log.Printf("%s%s Error unmarshaling %s params: %v", logging.LogTagLSP, logging.LogTagServer, req.Method(), err)

		return err
	}

	if params.Text != "" {
		s.setDocumentContent(params.TextDocument.URI, params.Text)
	}

	s.scheduleDiagnosticsPriority(params.TextDocument.URI)

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
				s.scheduleDiagnostics(change.URI)
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

	s.deleteDocumentContent(params.TextDocument.URI)
	s.scheduleDiagnostics(params.TextDocument.URI)

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

func (s *Server) setDocumentContent(uri protocol.DocumentURI, content string) {
	s.docMu.Lock()
	defer s.docMu.Unlock()
	s.documents[uri] = content
}

func (s *Server) getDocumentContent(uri protocol.DocumentURI) (string, bool) {
	s.docMu.RLock()
	defer s.docMu.RUnlock()
	content, exists := s.documents[uri]
	return content, exists
}

func (s *Server) deleteDocumentContent(uri protocol.DocumentURI) {
	s.docMu.Lock()
	defer s.docMu.Unlock()
	delete(s.documents, uri)
}

func (s *Server) scheduleDiagnostics(uri protocol.DocumentURI) {
	s.diagMu.Lock()

	if timer, exists := s.diagTimers[uri]; exists {
		timer.Stop()
	}

	if s.diagGen == nil {
		s.diagGen = make(map[protocol.DocumentURI]uint64)
	}
	s.diagGen[uri]++
	gen := s.diagGen[uri]

	s.diagTimers[uri] = time.AfterFunc(diagnosticsDebounceInterval, func() {
		s.diagMu.Lock()
		delete(s.diagTimers, uri)
		s.diagMu.Unlock()

		filePath := uri.Filename()
		diags := s.collectDiagnostics(context.Background(), filePath)

		s.diagMu.Lock()
		currentGen := s.diagGen[uri]
		s.diagMu.Unlock()
		if gen != currentGen {
			return
		}

		s.publishDiagnostics(context.Background(), uri, diags)
	})
	s.diagMu.Unlock()
}

func (s *Server) scheduleDiagnosticsPriority(uri protocol.DocumentURI) {
	s.diagMu.Lock()

	if timer, exists := s.diagTimers[uri]; exists {
		timer.Stop()
		delete(s.diagTimers, uri)
	}

	if s.diagGen == nil {
		s.diagGen = make(map[protocol.DocumentURI]uint64)
	}
	s.diagGen[uri]++
	gen := s.diagGen[uri]
	s.diagMu.Unlock()

	go func(u protocol.DocumentURI, g uint64) {
		filePath := u.Filename()
		diags := s.collectDiagnostics(context.Background(), filePath)

		s.diagMu.Lock()
		currentGen := s.diagGen[u]
		s.diagMu.Unlock()
		if g != currentGen {
			return
		}

		s.publishDiagnostics(context.Background(), u, diags)
	}(uri, gen)
}

func (s *Server) scheduleFormatting(ctx context.Context, reply jsonrpc2.Replier, params protocol.DocumentFormattingParams) {
	uri := params.TextDocument.URI

	s.fmtMu.Lock()

	if timer, exists := s.fmtTimers[uri]; exists {
		timer.Stop()
	}

	if s.fmtGen == nil {
		s.fmtGen = make(map[protocol.DocumentURI]uint64)
	}
	s.fmtGen[uri]++
	gen := s.fmtGen[uri]

	s.fmtTimers[uri] = time.AfterFunc(formattingDebounceInterval, func() {
		s.fmtMu.Lock()
		delete(s.fmtTimers, uri)
		currentGen := s.fmtGen[uri]
		s.fmtMu.Unlock()

		if gen != currentGen {
			_ = reply(ctx, []protocol.TextEdit{}, nil)
			return
		}

		filePath := uri.Filename()

		content, exists := s.getDocumentContent(uri)
		if !exists {
			fileContent, err := os.ReadFile(filePath)
			if err != nil {
				_ = reply(ctx, nil, fmt.Errorf("failed to read file: %w", err))
				return
			}
			content = string(fileContent)
		}

		formattingProviders := s.loadFormattingProviders()
		if len(formattingProviders) == 0 {
			_ = reply(ctx, []protocol.TextEdit{}, nil)
			return
		}

		provider := formattingProviders[0]
		formattedContent, err := provider.Format(ctx, filePath, content)
		if err != nil {
			_ = reply(ctx, []protocol.TextEdit{}, nil)
			return
		}

		if formattedContent == content {
			_ = reply(ctx, []protocol.TextEdit{}, nil)
			return
		}

		lines := strings.Split(content, "\n")
		endLine := uint32(len(lines) - 1)
		endCharacter := uint32(0)
		if len(lines) > 0 {
			endCharacter = uint32(len(lines[len(lines)-1]))
		}

		textEdits := []protocol.TextEdit{
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 0},
					End:   protocol.Position{Line: endLine, Character: endCharacter},
				},
				NewText: formattedContent,
			},
		}

		_ = reply(ctx, textEdits, nil)
	})
	s.fmtMu.Unlock()
}

func (s *Server) loadDiagnosticsProviders() []diagnostics.DiagnosticsProvider {
	// Return cached providers if already initialized
	if s.diagnosticsProviders != nil {
		return s.diagnosticsProviders
	}

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

	// Cache and return
	s.diagnosticsProviders = providers
	return s.diagnosticsProviders
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
	// Return cached providers if already initialized
	if s.formattingProviders != nil {
		return s.formattingProviders
	}

	// Initialize and cache
	s.formattingProviders = formatting.LoadFormattingProviders(s.serverConfig.DiagnosticsProviders)
	return s.formattingProviders
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

	s.scheduleFormatting(ctx, reply, params)
	return nil
}
