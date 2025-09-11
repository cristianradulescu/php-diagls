package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cristianradulescu/php-diagls/config"
	"github.com/cristianradulescu/php-diagls/internal/logging"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// Server represents the Language Server Protocol (LSP) server
type Server struct {
	conn jsonrpc2.Conn
	serverConfig *config.Config
}

// New creates a new LSP server instance
func New(conn jsonrpc2.Conn) *Server {
	s := &Server{
		conn: conn,
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
	case protocol.MethodShutdown:
		return s.handleShutdown(ctx, reply, req)
	case protocol.MethodExit:
		return s.handleExit(ctx, reply, req)
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
	// log.Printf("%s%s WorkspaceFolders=%s", logging.LogTagLSP, logging.LogTagServer, params.WorkspaceFolders)
	// log.Printf("%s%s RootURI=%s", logging.LogTagLSP, logging.LogTagServer, params.RootURI.Filename())
	// log.Printf("%s%s RootPath=%s", logging.LogTagLSP, logging.LogTagServer, params.RootPath)

	// Load configuration
	serverConfig, err := config.LoadConfig(params.WorkspaceFolders[0].Name)
	if err != nil {
		log.Fatalf("%s%s Error loading config: %v", logging.LogTagLSP, logging.LogTagServer, err)
	}
	s.serverConfig = serverConfig
	log.Printf("%s%s Loaded config: %s", logging.LogTagLSP, logging.LogTagServer, s.serverConfig.RawData)

	resp := protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				Change:    protocol.TextDocumentSyncKindFull,
				OpenClose: true,
				Save:      &protocol.SaveOptions{IncludeText: false},
			},
			ExecuteCommandProvider: &protocol.ExecuteCommandOptions{
				Commands: []string{
					"php-diagls.doSomething",
				},
			},
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    string(config.Name),
			Version: string(config.Version),
		},
	}

	log.Printf("%s%s Sending initialize response: %+v", logging.LogTagLSP, logging.LogTagServer, resp)

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
		case "php-diagls.showConfig":
			return s.handleShowConfigCommand(ctx, reply)
		default:
			return reply(ctx, nil, fmt.Errorf("unknown command: %s", params.Command))
		}
}

func (s *Server) handleShowConfigCommand(ctx context.Context, reply jsonrpc2.Replier) error {
	s.showWindowMessage(ctx, protocol.MessageTypeInfo, fmt.Sprintf("Current configuration: %s", s.serverConfig.RawData))

	return reply(ctx, nil, nil)
}

func (s *Server) handleShutdown(ctx context.Context, reply jsonrpc2.Replier, _ jsonrpc2.Request) error {
	log.Printf("%s%s Performing cleanup before shutdown", logging.LogTagLSP, logging.LogTagServer)

	return reply(ctx, nil, nil)
}

func (s *Server) handleExit(ctx context.Context, _ jsonrpc2.Replier, _ jsonrpc2.Request) error {
	log.Printf("%s%s Exiting server", logging.LogTagLSP, logging.LogTagServer)

	return s.conn.Close()
}

func (s *Server) showWindowMessage(ctx context.Context, messageType protocol.MessageType, message string) {
	params := &protocol.ShowMessageParams{Type: messageType, Message: message}
	if err := s.conn.Notify(ctx, protocol.MethodWindowShowMessage, params); err != nil {
		log.Printf("%s%s Failed to send window message: %v", logging.LogTagLSP, logging.LogTagServer, err)
	}
}
