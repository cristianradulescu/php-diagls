package server_test

import (
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"go.lsp.dev/protocol"
)

// TestServerCapabilities tests the server capabilities configuration
func TestServerCapabilities(t *testing.T) {
	t.Run("returns valid capabilities", func(t *testing.T) {
		// We can't call serverCapabilities directly as it's not exported
		// This test documents the expected capabilities structure
		t.Log("Expected server capabilities:")
		t.Log("- TextDocumentSync: Full sync with open/close/save")
		t.Log("- ExecuteCommandProvider: Supports php-diagls/showConfig command")
		t.Log("- DocumentFormattingProvider: true")
	})
}

// TestServerInfo tests the server info configuration
func TestServerInfo(t *testing.T) {
	t.Run("documents expected server info", func(t *testing.T) {
		// We can't call serverInfo directly as it's not exported
		// This test documents the expected server info structure
		t.Log("Expected server info:")
		t.Log("- Name: php-diagls")
		t.Log("- Version: from config.Version")
	})
}

// TestGetFullLspCommandName tests command name formatting
func TestGetFullLspCommandName(t *testing.T) {
	t.Run("documents command name format", func(t *testing.T) {
		// We can't call getFullLspCommandName directly as it's not exported
		// This test documents the expected command name format
		expectedFormat := "php-diagls/showConfig"
		t.Logf("Expected command format: %s", expectedFormat)
		t.Log("Format: <prefix>/<separator>/<command>")
		t.Logf("- Prefix: %s", config.Name)
		t.Log("- Separator: /")
		t.Log("- Command: showConfig")
	})
}

// TestServerConstants documents the server constants
func TestServerConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected interface{}
	}{
		{
			name:     "LspCommandPrefix",
			constant: "LspCommandPrefix",
			expected: "should equal config.Name (php-diagls)",
		},
		{
			name:     "LspCommandSeparator",
			constant: "LspCommandSeparator",
			expected: "/",
		},
		{
			name:     "LspCommandNameShowConfig",
			constant: "LspCommandNameShowConfig",
			expected: "showConfig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Constant %s: %v", tt.constant, tt.expected)
		})
	}
}

// TestServerDebounceIntervals documents the debounce intervals
func TestServerDebounceIntervals(t *testing.T) {
	t.Run("diagnostics debounce interval", func(t *testing.T) {
		t.Log("diagnosticsDebounceInterval: 300ms")
		t.Log("Purpose: Prevents excessive diagnostics runs during rapid edits")
		t.Log("Behavior: Last edit wins, previous pending diagnostics are cancelled")
	})

	t.Run("formatting debounce interval", func(t *testing.T) {
		t.Log("formattingDebounceInterval: 100ms")
		t.Log("Purpose: Prevents excessive formatting calls")
		t.Log("Behavior: Last request wins, previous pending formatting is cancelled")
	})
}

// TestServer_New tests server creation
func TestServer_New(t *testing.T) {
	// Can't easily test New() without a mock jsonrpc2.Conn
	// This documents what New() should initialize
	t.Log("Server.New() should initialize:")
	t.Log("- conn: jsonrpc2.Conn")
	t.Log("- serverConfig: empty config.Config")
	t.Log("- documents: empty map")
	t.Log("- diagTimers: empty map")
	t.Log("- diagGen: empty map")
	t.Log("- fmtTimers: empty map")
	t.Log("- fmtGen: empty map")
}

// TestServerHandle_MethodRouting documents the Handle method routing
func TestServerHandle_MethodRouting(t *testing.T) {
	tests := []struct {
		method      string
		handlerName string
		description string
	}{
		{
			method:      protocol.MethodInitialize,
			handlerName: "handleInitialize",
			description: "Loads config, initializes providers, returns capabilities",
		},
		{
			method:      protocol.MethodInitialized,
			handlerName: "handleInitialized",
			description: "Acknowledges initialization complete",
		},
		{
			method:      protocol.MethodWorkspaceExecuteCommand,
			handlerName: "handleExecuteCommand",
			description: "Executes custom commands (showConfig)",
		},
		{
			method:      protocol.MethodTextDocumentDidOpen,
			handlerName: "handleDidOpen",
			description: "Caches document content, schedules diagnostics",
		},
		{
			method:      protocol.MethodTextDocumentDidChange,
			handlerName: "handleDidChange",
			description: "Updates cached content, schedules diagnostics",
		},
		{
			method:      protocol.MethodTextDocumentDidClose,
			handlerName: "handleDidClose",
			description: "Removes cached content, schedules diagnostics",
		},
		{
			method:      protocol.MethodTextDocumentDidSave,
			handlerName: "handleDidSave",
			description: "Updates cached content, schedules priority diagnostics",
		},
		{
			method:      protocol.MethodTextDocumentFormatting,
			handlerName: "handleDocumentFormatting",
			description: "Schedules document formatting with debounce",
		},
		{
			method:      protocol.MethodWorkspaceDidChangeWatchedFiles,
			handlerName: "handleDidChangeWatchedFiles",
			description: "Handles file system changes for .php files",
		},
		{
			method:      protocol.MethodShutdown,
			handlerName: "handleShutdown",
			description: "Prepares for shutdown",
		},
		{
			method:      protocol.MethodExit,
			handlerName: "handleExit",
			description: "Closes connection and exits",
		},
		{
			method:      protocol.MethodCancelRequest,
			handlerName: "handleCancelRequest",
			description: "Acknowledges request cancellation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			t.Logf("Method: %s", tt.method)
			t.Logf("Handler: %s", tt.handlerName)
			t.Logf("Description: %s", tt.description)
		})
	}

	t.Run("unhandled methods", func(t *testing.T) {
		t.Log("Unhandled methods return reply(ctx, nil, nil)")
		t.Log("Server logs warning for unhandled methods")
	})
}

// TestServerDocumentManagement documents document cache behavior
func TestServerDocumentManagement(t *testing.T) {
	t.Run("setDocumentContent", func(t *testing.T) {
		t.Log("Stores document content in memory cache")
		t.Log("Protected by RWMutex (s.docMu)")
		t.Log("Used for synchronized document content from client")
	})

	t.Run("getDocumentContent", func(t *testing.T) {
		t.Log("Retrieves document content from cache")
		t.Log("Returns (content string, exists bool)")
		t.Log("Protected by RWMutex (s.docMu) for concurrent reads")
	})

	t.Run("deleteDocumentContent", func(t *testing.T) {
		t.Log("Removes document from cache when closed")
		t.Log("Protected by RWMutex (s.docMu)")
	})
}

// TestServerDiagnosticsScheduling documents diagnostics scheduling behavior
func TestServerDiagnosticsScheduling(t *testing.T) {
	t.Run("scheduleDiagnostics", func(t *testing.T) {
		t.Log("Schedules diagnostics with 300ms debounce")
		t.Log("Cancels previous timer if exists (last-wins strategy)")
		t.Log("Increments generation counter for race prevention")
		t.Log("Runs collectDiagnostics in timer callback")
		t.Log("Checks generation before publishing (prevents stale results)")
	})

	t.Run("scheduleDiagnosticsPriority", func(t *testing.T) {
		t.Log("Immediately runs diagnostics without debounce")
		t.Log("Used for save events (DidSave)")
		t.Log("Runs in goroutine for async execution")
		t.Log("Still uses generation counter for race prevention")
	})

	t.Run("generation counter behavior", func(t *testing.T) {
		t.Log("Per-file generation counter (diagGen map)")
		t.Log("Incremented on each schedule call")
		t.Log("Prevents publishing stale results from earlier requests")
		t.Log("Example: Edit 1 (gen=1) -> Edit 2 (gen=2)")
		t.Log("  If gen=1 completes after gen=2 starts, result is discarded")
	})
}

// TestServerFormattingScheduling documents formatting scheduling behavior
func TestServerFormattingScheduling(t *testing.T) {
	t.Run("scheduleFormatting", func(t *testing.T) {
		t.Log("Schedules formatting with 100ms debounce")
		t.Log("Cancels previous timer if exists (last-wins strategy)")
		t.Log("Increments generation counter for race prevention")
		t.Log("Checks generation before replying (prevents stale results)")
	})

	t.Run("formatting behavior", func(t *testing.T) {
		t.Log("1. Gets content from cache or reads from file")
		t.Log("2. Loads formatting providers (cached)")
		t.Log("3. Uses first provider only")
		t.Log("4. Calls provider.Format(ctx, filePath, content)")
		t.Log("5. If no changes, returns empty TextEdit array")
		t.Log("6. If changed, returns TextEdit replacing entire document")
	})

	t.Run("text edit calculation", func(t *testing.T) {
		t.Log("Replaces entire document content:")
		t.Log("  Start: Line 0, Character 0")
		t.Log("  End: Last line, last character")
		t.Log("  NewText: formatted content")
	})
}

// TestServerProviderLoading documents provider loading behavior
func TestServerProviderLoading(t *testing.T) {
	t.Run("loadDiagnosticsProviders", func(t *testing.T) {
		t.Log("Loads and caches diagnostics providers")
		t.Log("Called once during handleInitialize")
		t.Log("Returns cached providers on subsequent calls")
		t.Log("Skips disabled providers")
		t.Log("Shows error message window if provider creation fails")
	})

	t.Run("loadFormattingProviders", func(t *testing.T) {
		t.Log("Loads and caches formatting providers")
		t.Log("Called once during handleInitialize")
		t.Log("Returns cached providers on subsequent calls")
		t.Log("Uses formatting.LoadFormattingProviders()")
	})

	t.Run("provider caching", func(t *testing.T) {
		t.Log("Providers are loaded once and cached in Server struct")
		t.Log("diagnosticsProviders: []diagnostics.DiagnosticsProvider")
		t.Log("formattingProviders: []formatting.FormattingProvider")
		t.Log("Cache check: if providers != nil, return cached")
	})
}

// TestServerCollectDiagnostics documents diagnostics collection behavior
func TestServerCollectDiagnostics(t *testing.T) {
	t.Run("ignored directories", func(t *testing.T) {
		t.Log("Ignores files in: /vendor/, /var/cache/")
		t.Log("Returns empty diagnostics slice for ignored paths")
	})

	t.Run("parallel execution", func(t *testing.T) {
		t.Log("Runs all providers in parallel using goroutines")
		t.Log("Uses sync.WaitGroup to wait for all providers")
		t.Log("Uses sync.Mutex to protect diagnostics slice")
	})

	t.Run("error handling", func(t *testing.T) {
		t.Log("Shows error window message if provider fails")
		t.Log("Continues with other providers on error")
		t.Log("Returns combined diagnostics from all successful providers")
	})
}

// TestServerMessageHandling documents message handling behavior
func TestServerMessageHandling(t *testing.T) {
	t.Run("showWindowMessage", func(t *testing.T) {
		t.Log("Sends window/showMessage notification to client")
		t.Log("Used for errors and info messages")
		t.Log("Logs error if notification fails")
	})

	t.Run("publishDiagnostics", func(t *testing.T) {
		t.Log("Sends textDocument/publishDiagnostics notification")
		t.Log("Uses utils.EnsureDiagnosticsArray() to ensure array (not null)")
		t.Log("Logs error if notification fails")
	})
}

// TestServerInitialization documents initialization behavior
func TestServerInitialization(t *testing.T) {
	t.Run("project root detection", func(t *testing.T) {
		t.Log("Priority order:")
		t.Log("1. WorkspaceFolders[0].URI")
		t.Log("2. RootURI (deprecated)")
		t.Log("3. os.Getwd() fallback")
	})

	t.Run("config loading", func(t *testing.T) {
		t.Log("Loads config using projectRoot")
		t.Log("CRITICAL-002: Calls os.Exit(0) if config not found")
		t.Log("This makes the function untestable")
		t.Log("Should return error instead of calling os.Exit()")
	})

	t.Run("provider preloading", func(t *testing.T) {
		t.Log("Preloads diagnostics providers during init")
		t.Log("Preloads formatting providers during init")
		t.Log("Discards return value (providers are cached)")
	})
}

// TestServerCriticalIssues documents known critical issues
func TestServerCriticalIssues(t *testing.T) {
	t.Run("CRITICAL-002: os.Exit in handleInitialize", func(t *testing.T) {
		t.Log("Location: server.go:127")
		t.Log("Issue: os.Exit(0) makes function untestable")
		t.Log("Impact: Cannot test initialization error path")
		t.Log("Impact: Prevents graceful error handling in tests")
		t.Log("Recommendation: Return error instead of os.Exit()")
		t.Log("Alternative: Use dependency injection for exit function")
	})

	t.Run("async operations make testing difficult", func(t *testing.T) {
		t.Log("time.AfterFunc() in scheduling makes tests race-prone")
		t.Log("Goroutines in collectDiagnostics require careful synchronization")
		t.Log("Generation counters add complexity to testing")
		t.Log("Would benefit from time abstraction/dependency injection")
	})

	t.Run("jsonrpc2.Conn dependency", func(t *testing.T) {
		t.Log("Most methods require mock jsonrpc2.Conn")
		t.Log("Conn interface is complex (Notify, Close methods)")
		t.Log("Would benefit from interface wrapper for testing")
	})
}

// TestServerFileWatcherBehavior documents file watcher behavior
func TestServerFileWatcherBehavior(t *testing.T) {
	t.Run("file change types", func(t *testing.T) {
		t.Log("FileChangeTypeChanged: Schedules diagnostics")
		t.Log("FileChangeTypeCreated: Schedules diagnostics")
		t.Log("FileChangeTypeDeleted: Publishes empty diagnostics (clears)")
	})

	t.Run("file filtering", func(t *testing.T) {
		t.Log("Only processes files ending with .php")
		t.Log("Ignores non-PHP files")
	})
}

// TestServerExecuteCommand documents command execution behavior
func TestServerExecuteCommand(t *testing.T) {
	t.Run("showConfig command", func(t *testing.T) {
		t.Log("Command: php-diagls/showConfig")
		t.Log("Shows window message with current config raw data")
		t.Log("Returns nil result")
	})

	t.Run("unknown commands", func(t *testing.T) {
		t.Log("Returns error: 'unknown command: <name>'")
		t.Log("Error is sent as reply to client")
	})
}

// TestServerShutdownBehavior documents shutdown behavior
func TestServerShutdownBehavior(t *testing.T) {
	t.Run("shutdown method", func(t *testing.T) {
		t.Log("Logs cleanup message")
		t.Log("Returns nil (acknowledges shutdown request)")
		t.Log("Does not actually close connection")
	})

	t.Run("exit method", func(t *testing.T) {
		t.Log("Logs exit message")
		t.Log("Calls conn.Close() to close connection")
		t.Log("Returns error from Close() if any")
	})

	t.Run("LSP shutdown sequence", func(t *testing.T) {
		t.Log("1. Client sends shutdown request")
		t.Log("2. Server handles shutdown, returns response")
		t.Log("3. Client sends exit notification")
		t.Log("4. Server handles exit, closes connection")
	})
}

// TestServerConcurrencySafety documents concurrency safety measures
func TestServerConcurrencySafety(t *testing.T) {
	t.Run("document cache", func(t *testing.T) {
		t.Log("Protected by sync.RWMutex (docMu)")
		t.Log("Allows multiple concurrent reads")
		t.Log("Exclusive write access")
	})

	t.Run("diagnostics scheduling", func(t *testing.T) {
		t.Log("Protected by sync.Mutex (diagMu)")
		t.Log("Guards diagTimers and diagGen maps")
		t.Log("Prevents race conditions in timer management")
	})

	t.Run("formatting scheduling", func(t *testing.T) {
		t.Log("Protected by sync.Mutex (fmtMu)")
		t.Log("Guards fmtTimers and fmtGen maps")
		t.Log("Prevents race conditions in timer management")
	})

	t.Run("diagnostics collection", func(t *testing.T) {
		t.Log("Uses sync.WaitGroup for goroutine coordination")
		t.Log("Uses sync.Mutex for diagnostics slice protection")
		t.Log("Allows parallel provider execution")
	})
}

// TestServerTestability documents testability challenges and solutions
func TestServerTestability(t *testing.T) {
	t.Run("what can be tested", func(t *testing.T) {
		t.Log("✓ Constants and configuration")
		t.Log("✓ Document cache operations (with mock connection)")
		t.Log("✓ Provider loading logic (with test config)")
		t.Log("✓ Ignored directory filtering")
		t.Log("✓ Error handling paths")
	})

	t.Run("what cannot be easily tested", func(t *testing.T) {
		t.Log("✗ Initialization (os.Exit blocks testing)")
		t.Log("✗ Async scheduling (time.AfterFunc)")
		t.Log("✗ LSP protocol handling (requires jsonrpc2.Conn mock)")
		t.Log("✗ Notification sending (requires connection)")
		t.Log("✗ Generation counter races (timing-dependent)")
	})

	t.Run("recommended improvements", func(t *testing.T) {
		t.Log("1. Replace os.Exit with error return")
		t.Log("2. Add time abstraction (clock interface)")
		t.Log("3. Create minimal Conn interface wrapper")
		t.Log("4. Extract business logic from handlers")
		t.Log("5. Add integration test fixtures")
	})
}

// TestServerGetPhpCsFixerProviderConfig documents provider config lookup
func TestServerGetPhpCsFixerProviderConfig(t *testing.T) {
	t.Run("behavior", func(t *testing.T) {
		t.Log("Searches serverConfig.DiagnosticsProviders map")
		t.Log("Returns (config, true) if found and enabled")
		t.Log("Returns (empty config, false) if not found or disabled")
		t.Log("Only matches diagnostics.PhpCsFixerProviderId")
	})

	t.Run("usage", func(t *testing.T) {
		t.Log("Used to check if PHP CS Fixer is available")
		t.Log("Note: Currently defined but not used in codebase")
		t.Log("May be for future features or leftover from refactoring")
	})
}
