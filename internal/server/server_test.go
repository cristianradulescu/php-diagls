package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"github.com/cristianradulescu/php-diagls/internal/diagnostics"
	"github.com/cristianradulescu/php-diagls/internal/formatting"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// ---- test doubles -----------------------------------------------------------

type notification struct {
	method string
	params interface{}
}

// fakeConn records every notification the server sends to the client.
type fakeConn struct {
	mu     sync.Mutex
	sent   []notification
	closed bool
	wake   chan struct{}
}

func newFakeConn() *fakeConn {
	return &fakeConn{wake: make(chan struct{}, 64)}
}

func (c *fakeConn) Notify(_ context.Context, method string, params interface{}) error {
	c.mu.Lock()
	c.sent = append(c.sent, notification{method: method, params: params})
	c.mu.Unlock()
	select {
	case c.wake <- struct{}{}:
	default:
	}
	return nil
}

func (c *fakeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeConn) notifications(method string) []notification {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []notification
	for _, n := range c.sent {
		if n.method == method {
			out = append(out, n)
		}
	}
	return out
}

// waitFor polls until cond holds or the deadline passes.
func (c *fakeConn) waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		if cond() {
			return
		}
		select {
		case <-c.wake:
		case <-time.After(10 * time.Millisecond):
		case <-deadline:
			t.Fatalf("condition not met within %v", timeout)
		}
	}
}

func (c *fakeConn) published(uri protocol.DocumentURI) []protocol.PublishDiagnosticsParams {
	var out []protocol.PublishDiagnosticsParams
	for _, n := range c.notifications(protocol.MethodTextDocumentPublishDiagnostics) {
		p := n.params.(protocol.PublishDiagnosticsParams)
		if p.URI == uri {
			out = append(out, p)
		}
	}
	return out
}

func (c *fakeConn) messages() []string {
	var out []string
	for _, n := range c.notifications(protocol.MethodWindowShowMessage) {
		out = append(out, n.params.(*protocol.ShowMessageParams).Message)
	}
	return out
}

// fakeProvider is a scriptable DiagnosticsProvider.
type fakeProvider struct {
	id      string
	analyze func(ctx context.Context, filePath string) ([]protocol.Diagnostic, error)
}

func (p *fakeProvider) Id() string   { return p.id }
func (p *fakeProvider) Name() string { return p.id }
func (p *fakeProvider) Analyze(ctx context.Context, filePath string) ([]protocol.Diagnostic, error) {
	return p.analyze(ctx, filePath)
}

// fakeFormatter is a scriptable FormattingProvider.
type fakeFormatter struct {
	format func(ctx context.Context, filePath, content string) (string, error)
}

func (f *fakeFormatter) Id() string   { return "fake" }
func (f *fakeFormatter) Name() string { return "fake" }
func (f *fakeFormatter) Format(ctx context.Context, filePath, content string) (string, error) {
	return f.format(ctx, filePath, content)
}

// reply captures a single jsonrpc2 reply.
type reply struct {
	mu     sync.Mutex
	calls  int
	result interface{}
	err    error
	done   chan struct{}
}

func newReply() *reply { return &reply{done: make(chan struct{}, 1)} }

func (r *reply) fn(_ context.Context, result interface{}, err error) error {
	r.mu.Lock()
	r.calls++
	r.result, r.err = result, err
	r.mu.Unlock()
	select {
	case r.done <- struct{}{}:
	default:
	}
	return nil
}

func (r *reply) wait(t *testing.T) {
	t.Helper()
	select {
	case <-r.done:
	case <-time.After(5 * time.Second):
		t.Fatal("no reply within 5s")
	}
}

func diag(line uint32, msg string) protocol.Diagnostic {
	return protocol.Diagnostic{
		Range:   protocol.Range{Start: protocol.Position{Line: line}, End: protocol.Position{Line: line}},
		Message: msg,
	}
}

// newTestServer returns a server with providers already "loaded" (so no
// docker validation happens) and the given diagnostics providers installed.
func newTestServer(t *testing.T, providers ...diagnostics.DiagnosticsProvider) (*Server, *fakeConn) {
	t.Helper()
	conn := newFakeConn()
	s := New(conn)
	s.exit = func(code int) { t.Fatalf("unexpected exit(%d)", code) }
	if providers == nil {
		providers = []diagnostics.DiagnosticsProvider{}
	}
	s.diagnosticsProviders = providers
	s.formattingProviders = []formatting.FormattingProvider{}
	return s, conn
}

func call(t *testing.T, s *Server, method string, params interface{}, r *reply) {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	req, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(1), method, json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	var replier jsonrpc2.Replier
	if r != nil {
		replier = r.fn
	} else {
		replier = func(context.Context, interface{}, error) error { return nil }
	}
	if err := s.Handle(context.Background(), replier, req); err != nil {
		t.Fatalf("Handle(%s) returned error: %v", method, err)
	}
}

func notify(t *testing.T, s *Server, method string, params interface{}) {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	req, err := jsonrpc2.NewNotification(method, json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Handle(context.Background(), func(context.Context, interface{}, error) error { return nil }, req); err != nil {
		t.Fatalf("Handle(%s) returned error: %v", method, err)
	}
}

func fileURI(dir, name string) protocol.DocumentURI {
	return protocol.DocumentURI("file://" + filepath.Join(dir, name))
}

// ---- capabilities ------------------------------------------------------------

func TestServerCapabilities(t *testing.T) {
	caps := serverCapabilities()
	if !caps.DocumentFormattingProvider.(bool) {
		t.Error("formatting should be advertised")
	}
	sync := caps.TextDocumentSync.(*protocol.TextDocumentSyncOptions)
	if sync.Change != protocol.TextDocumentSyncKindFull || !sync.OpenClose || sync.Save == nil {
		t.Errorf("unexpected sync options: %+v", sync)
	}
	cmds := caps.ExecuteCommandProvider.Commands
	if len(cmds) != 1 || cmds[0] != "php-diagls/showConfig" {
		t.Errorf("unexpected commands: %v", cmds)
	}
	if info := serverInfo(); info.Name != config.Name || info.Version != config.Version {
		t.Errorf("unexpected server info: %+v", info)
	}
}

// ---- initialize -----------------------------------------------------------------

func writeConfig(t *testing.T, dir string) {
	t.Helper()
	cfg := `{"diagnosticsProviders":{"phplint":{"enabled":false,"container":"c","path":"/usr/bin/php"}}}`
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInitialize_LoadsConfigFromWorkspaceFolder(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir)
	conn := newFakeConn()
	s := New(conn)
	s.exit = func(code int) { t.Fatalf("unexpected exit(%d)", code) }

	r := newReply()
	call(t, s, protocol.MethodInitialize, protocol.InitializeParams{
		WorkspaceFolders: []protocol.WorkspaceFolder{{URI: "file://" + dir}},
	}, r)

	if r.calls != 1 || r.err != nil {
		t.Fatalf("expected one successful reply, got calls=%d err=%v", r.calls, r.err)
	}
	if _, ok := r.result.(protocol.InitializeResult); !ok {
		t.Fatalf("unexpected result type %T", r.result)
	}
	if !s.serverConfig.IsInitialized() {
		t.Error("config should be initialized")
	}
	if len(s.diagnosticsProviders) != 0 {
		t.Errorf("disabled provider should not be loaded, got %d", len(s.diagnosticsProviders))
	}
}

func TestInitialize_DecodesPercentEncodedRoot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "my project")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, dir)
	s := New(newFakeConn())
	s.exit = func(code int) { t.Fatalf("unexpected exit(%d): root with a space was not decoded", code) }

	call(t, s, protocol.MethodInitialize, protocol.InitializeParams{
		RootURI: protocol.DocumentURI("file://" + strings.ReplaceAll(dir, " ", "%20")),
	}, newReply())

	if !s.serverConfig.IsInitialized() {
		t.Error("config should be initialized")
	}
}

func TestInitialize_MissingConfigExits(t *testing.T) {
	s := New(newFakeConn())
	exited := -1
	s.exit = func(code int) { exited = code }

	r := newReply()
	call(t, s, protocol.MethodInitialize, protocol.InitializeParams{RootURI: protocol.DocumentURI("file://" + t.TempDir())}, r)

	if exited != 0 {
		t.Errorf("expected exit(0), got %d", exited)
	}
	if r.err == nil {
		t.Error("expected an error reply when config is missing")
	}
}

// ---- document lifecycle -------------------------------------------------------

func TestDidOpen_CachesContentAndPublishesDiagnostics(t *testing.T) {
	dir := t.TempDir()
	uri := fileURI(dir, "a.php")
	var mu sync.Mutex
	var seenPath string
	s, conn := newTestServer(t, &fakeProvider{id: "p", analyze: func(_ context.Context, path string) ([]protocol.Diagnostic, error) {
		mu.Lock()
		seenPath = path
		mu.Unlock()
		return []protocol.Diagnostic{diag(3, "boom")}, nil
	}})

	notify(t, s, protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: uri, Text: "<?php\n"},
	})

	if content, ok := s.getDocumentContent(uri); !ok || content != "<?php\n" {
		t.Errorf("content not cached: %q %v", content, ok)
	}
	conn.waitFor(t, 3*time.Second, func() bool { return len(conn.published(uri)) == 1 })
	got := conn.published(uri)[0]
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Message != "boom" {
		t.Errorf("unexpected diagnostics: %+v", got.Diagnostics)
	}
	mu.Lock()
	defer mu.Unlock()
	if seenPath != filepath.Join(dir, "a.php") {
		t.Errorf("provider got path %q", seenPath)
	}
}

func TestScheduleDiagnostics_DebouncesToSingleRun(t *testing.T) {
	uri := fileURI(t.TempDir(), "a.php")
	var mu sync.Mutex
	runs := 0
	s, conn := newTestServer(t, &fakeProvider{id: "p", analyze: func(context.Context, string) ([]protocol.Diagnostic, error) {
		mu.Lock()
		runs++
		mu.Unlock()
		return nil, nil
	}})

	for i := 0; i < 5; i++ {
		s.scheduleDiagnostics(uri)
	}

	conn.waitFor(t, 3*time.Second, func() bool { return len(conn.published(uri)) == 1 })
	time.Sleep(2 * diagnosticsDebounceInterval)
	mu.Lock()
	defer mu.Unlock()
	if runs != 1 {
		t.Errorf("expected exactly one analysis, got %d", runs)
	}
	if len(conn.published(uri)) != 1 {
		t.Errorf("expected exactly one publish, got %d", len(conn.published(uri)))
	}
}

func TestDidSave_RunsImmediatelyAndSupersedesDebounced(t *testing.T) {
	uri := fileURI(t.TempDir(), "a.php")
	s, conn := newTestServer(t, &fakeProvider{id: "p", analyze: func(context.Context, string) ([]protocol.Diagnostic, error) {
		return []protocol.Diagnostic{diag(0, "x")}, nil
	}})

	s.scheduleDiagnostics(uri)
	start := time.Now()
	notify(t, s, protocol.MethodTextDocumentDidSave, protocol.DidSaveTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})

	conn.waitFor(t, 3*time.Second, func() bool { return len(conn.published(uri)) >= 1 })
	if time.Since(start) >= diagnosticsDebounceInterval {
		t.Error("save path should not wait for the debounce interval")
	}
	// The pending debounced timer was stopped, so no second publish follows.
	time.Sleep(2 * diagnosticsDebounceInterval)
	if n := len(conn.published(uri)); n != 1 {
		t.Errorf("expected 1 publish, got %d", n)
	}
}

func TestStaleGenerationIsDiscarded(t *testing.T) {
	uri := fileURI(t.TempDir(), "a.php")
	release := make(chan struct{})
	var mu sync.Mutex
	calls := 0
	s, conn := newTestServer(t, &fakeProvider{id: "p", analyze: func(ctx context.Context, _ string) ([]protocol.Diagnostic, error) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			// First run blocks until the second one has been scheduled.
			select {
			case <-release:
			case <-ctx.Done():
			}
			return []protocol.Diagnostic{diag(0, "stale")}, nil
		}
		return []protocol.Diagnostic{diag(0, "fresh")}, nil
	}})

	s.scheduleDiagnosticsPriority(uri)
	conn.waitFor(t, 3*time.Second, func() bool { mu.Lock(); defer mu.Unlock(); return calls == 1 })
	s.scheduleDiagnosticsPriority(uri) // cancels run 1's context, bumps generation
	close(release)

	conn.waitFor(t, 3*time.Second, func() bool { return len(conn.published(uri)) >= 1 })
	time.Sleep(100 * time.Millisecond)
	pubs := conn.published(uri)
	if len(pubs) != 1 || pubs[0].Diagnostics[0].Message != "fresh" {
		t.Errorf("expected only the fresh result, got %+v", pubs)
	}
}

func TestDidClose_ClearsStateAndDiagnostics(t *testing.T) {
	uri := fileURI(t.TempDir(), "a.php")
	s, conn := newTestServer(t, &fakeProvider{id: "p", analyze: func(ctx context.Context, _ string) ([]protocol.Diagnostic, error) {
		<-ctx.Done()
		return []protocol.Diagnostic{diag(0, "late")}, nil
	}})

	s.setDocumentContent(uri, "<?php")
	s.scheduleDiagnosticsPriority(uri)
	s.scheduleDiagnostics(uri)
	notify(t, s, protocol.MethodTextDocumentDidClose, protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})

	if _, ok := s.getDocumentContent(uri); ok {
		t.Error("document content should be dropped on close")
	}
	pubs := conn.published(uri)
	if len(pubs) != 1 || len(pubs[0].Diagnostics) != 0 {
		t.Fatalf("expected one empty publish on close, got %+v", pubs)
	}
	time.Sleep(2 * diagnosticsDebounceInterval)
	if n := len(conn.published(uri)); n != 1 {
		t.Errorf("in-flight/pending analyses must not publish after close, got %d publishes", n)
	}
	s.diagMu.Lock()
	_, hasTimer := s.diagTimers[uri]
	_, hasGen := s.diagGen[uri]
	_, hasCancel := s.diagCancel[uri]
	s.diagMu.Unlock()
	if hasTimer || hasGen || hasCancel {
		t.Errorf("per-URI state should be cleaned up: timer=%v gen=%v cancel=%v", hasTimer, hasGen, hasCancel)
	}
}

func TestDidChange_UpdatesCacheWithoutAnalysis(t *testing.T) {
	uri := fileURI(t.TempDir(), "a.php")
	s, conn := newTestServer(t, &fakeProvider{id: "p", analyze: func(context.Context, string) ([]protocol.Diagnostic, error) {
		t.Error("didChange must not trigger analysis")
		return nil, nil
	}})

	notify(t, s, protocol.MethodTextDocumentDidChange, protocol.DidChangeTextDocumentParams{
		TextDocument:   protocol.VersionedTextDocumentIdentifier{TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri}},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{Text: "v1"}, {Text: "v2"}},
	})

	if content, _ := s.getDocumentContent(uri); content != "v2" {
		t.Errorf("expected last change to win, got %q", content)
	}
	time.Sleep(2 * diagnosticsDebounceInterval)
	if len(conn.published(uri)) != 0 {
		t.Error("no diagnostics expected on didChange")
	}
}

func TestDidChangeWatchedFiles(t *testing.T) {
	dir := t.TempDir()
	changed, deleted, other := fileURI(dir, "c.php"), fileURI(dir, "d.php"), fileURI(dir, "x.txt")
	s, conn := newTestServer(t, &fakeProvider{id: "p", analyze: func(_ context.Context, path string) ([]protocol.Diagnostic, error) {
		return []protocol.Diagnostic{diag(0, filepath.Base(path))}, nil
	}})

	notify(t, s, protocol.MethodWorkspaceDidChangeWatchedFiles, protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{
			{URI: changed, Type: protocol.FileChangeTypeChanged},
			{URI: deleted, Type: protocol.FileChangeTypeDeleted},
			{URI: other, Type: protocol.FileChangeTypeChanged},
		},
	})

	conn.waitFor(t, 3*time.Second, func() bool { return len(conn.published(changed)) == 1 })
	if len(conn.published(deleted)) != 1 || len(conn.published(deleted)[0].Diagnostics) != 0 {
		t.Error("deleted file should get an empty publish")
	}
	time.Sleep(2 * diagnosticsDebounceInterval)
	if len(conn.published(other)) != 0 {
		t.Error("non-PHP files should be ignored")
	}
}

// ---- collectDiagnostics ---------------------------------------------------------

func TestCollectDiagnostics_MergesProvidersAndReportsErrors(t *testing.T) {
	s, conn := newTestServer(t,
		&fakeProvider{id: "ok", analyze: func(context.Context, string) ([]protocol.Diagnostic, error) {
			return []protocol.Diagnostic{diag(1, "one")}, nil
		}},
		&fakeProvider{id: "partial", analyze: func(context.Context, string) ([]protocol.Diagnostic, error) {
			return []protocol.Diagnostic{diag(2, "two")}, errors.New("config broken")
		}},
	)

	diags := s.collectDiagnostics(context.Background(), "/p/src/a.php")

	if len(diags) != 2 {
		t.Errorf("partial results should be kept, got %d diagnostics", len(diags))
	}
	msgs := conn.messages()
	if len(msgs) != 1 || !strings.Contains(msgs[0], "partial") || !strings.Contains(msgs[0], "config broken") {
		t.Errorf("expected one error message naming the provider, got %v", msgs)
	}
}

func TestCollectDiagnostics_SilentOnCancellation(t *testing.T) {
	s, conn := newTestServer(t, &fakeProvider{id: "p", analyze: func(ctx context.Context, _ string) ([]protocol.Diagnostic, error) {
		return nil, ctx.Err()
	}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.collectDiagnostics(ctx, "/p/src/a.php")

	if msgs := conn.messages(); len(msgs) != 0 {
		t.Errorf("cancelled runs must not nag the user, got %v", msgs)
	}
}

func TestCollectDiagnostics_SkipsIgnoredDirs(t *testing.T) {
	s, _ := newTestServer(t, &fakeProvider{id: "p", analyze: func(context.Context, string) ([]protocol.Diagnostic, error) {
		t.Error("provider should not run for ignored paths")
		return nil, nil
	}})
	for _, p := range []string{"/p/vendor/x.php", "/p/var/cache/y.php"} {
		if got := s.collectDiagnostics(context.Background(), p); len(got) != 0 {
			t.Errorf("%s: expected no diagnostics", p)
		}
	}
}

func TestRunDiagnostics_BoundedConcurrency(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	running, peak := 0, 0
	s, conn := newTestServer(t, &fakeProvider{id: "p", analyze: func(context.Context, string) ([]protocol.Diagnostic, error) {
		mu.Lock()
		running++
		if running > peak {
			peak = running
		}
		mu.Unlock()
		time.Sleep(30 * time.Millisecond)
		mu.Lock()
		running--
		mu.Unlock()
		return nil, nil
	}})

	const files = 12
	for i := 0; i < files; i++ {
		s.scheduleDiagnosticsPriority(fileURI(dir, "f"+string(rune('a'+i))+".php"))
	}

	conn.waitFor(t, 5*time.Second, func() bool {
		return len(conn.notifications(protocol.MethodTextDocumentPublishDiagnostics)) == files
	})
	mu.Lock()
	defer mu.Unlock()
	if peak > maxConcurrentAnalyses {
		t.Errorf("peak concurrency %d exceeds cap %d", peak, maxConcurrentAnalyses)
	}
	if peak < 2 {
		t.Errorf("expected some parallelism, peak was %d", peak)
	}
}

// ---- formatting -------------------------------------------------------------------

func formatParams(uri protocol.DocumentURI) protocol.DocumentFormattingParams {
	return protocol.DocumentFormattingParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri}}
}

func TestFormatting_NoProvidersRepliesEmpty(t *testing.T) {
	uri := fileURI(t.TempDir(), "a.php")
	s, _ := newTestServer(t)
	s.setDocumentContent(uri, "<?php")

	r := newReply()
	call(t, s, protocol.MethodTextDocumentFormatting, formatParams(uri), r)
	r.wait(t)

	if edits, ok := r.result.([]protocol.TextEdit); !ok || len(edits) != 0 || r.err != nil {
		t.Errorf("expected empty edits, got %+v err=%v", r.result, r.err)
	}
}

func TestFormatting_UsesCachedContentAndRepliesWithEdit(t *testing.T) {
	uri := fileURI(t.TempDir(), "a.php")
	s, _ := newTestServer(t)
	s.formattingProviders = []formatting.FormattingProvider{&fakeFormatter{format: func(_ context.Context, _ string, content string) (string, error) {
		return strings.ReplaceAll(content, "array()", "[]"), nil
	}}}
	s.setDocumentContent(uri, "<?php\n$a = array();\n")

	r := newReply()
	call(t, s, protocol.MethodTextDocumentFormatting, formatParams(uri), r)
	r.wait(t)

	edits := r.result.([]protocol.TextEdit)
	if len(edits) != 1 || edits[0].NewText != "<?php\n$a = [];\n" {
		t.Errorf("unexpected edits: %+v", edits)
	}
}

func TestFormatting_ReadsFromDiskWhenNotOpen(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.php"), []byte("on disk"), 0o644); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var seen string
	s, _ := newTestServer(t)
	s.formattingProviders = []formatting.FormattingProvider{&fakeFormatter{format: func(_ context.Context, _ string, content string) (string, error) {
		mu.Lock()
		seen = content
		mu.Unlock()
		return content, nil
	}}}

	r := newReply()
	call(t, s, protocol.MethodTextDocumentFormatting, formatParams(fileURI(dir, "a.php")), r)
	r.wait(t)

	mu.Lock()
	defer mu.Unlock()
	if seen != "on disk" {
		t.Errorf("expected disk content, provider saw %q", seen)
	}
}

func TestFormatting_MissingFileRepliesError(t *testing.T) {
	s, _ := newTestServer(t)
	s.formattingProviders = []formatting.FormattingProvider{&fakeFormatter{format: func(_ context.Context, _ string, c string) (string, error) { return c, nil }}}

	r := newReply()
	call(t, s, protocol.MethodTextDocumentFormatting, formatParams(fileURI(t.TempDir(), "missing.php")), r)
	r.wait(t)

	if r.err == nil {
		t.Error("expected an error reply for an unreadable file")
	}
}

func TestFormatting_ProviderErrorRepliesEmpty(t *testing.T) {
	uri := fileURI(t.TempDir(), "a.php")
	s, _ := newTestServer(t)
	s.formattingProviders = []formatting.FormattingProvider{&fakeFormatter{format: func(context.Context, string, string) (string, error) {
		return "", errors.New("php-cs-fixer exploded")
	}}}
	s.setDocumentContent(uri, "<?php")

	r := newReply()
	call(t, s, protocol.MethodTextDocumentFormatting, formatParams(uri), r)
	r.wait(t)

	if edits, ok := r.result.([]protocol.TextEdit); !ok || len(edits) != 0 || r.err != nil {
		t.Errorf("formatter failure should yield no edits and no error, got %+v err=%v", r.result, r.err)
	}
}

func TestFormatting_ConcurrentRequestsEachGetOneReply(t *testing.T) {
	uri := fileURI(t.TempDir(), "a.php")
	s, _ := newTestServer(t)
	s.formattingProviders = []formatting.FormattingProvider{&fakeFormatter{format: func(_ context.Context, _ string, content string) (string, error) {
		time.Sleep(20 * time.Millisecond)
		return content + "\n", nil
	}}}
	s.setDocumentContent(uri, "<?php")

	replies := []*reply{newReply(), newReply(), newReply()}
	for _, r := range replies {
		call(t, s, protocol.MethodTextDocumentFormatting, formatParams(uri), r)
	}
	for i, r := range replies {
		r.wait(t)
		r.mu.Lock()
		if r.calls != 1 {
			t.Errorf("request %d replied %d times, want exactly 1", i, r.calls)
		}
		r.mu.Unlock()
	}
}

// ---- misc handlers ----------------------------------------------------------------

func TestExecuteCommand(t *testing.T) {
	s, conn := newTestServer(t)
	s.serverConfig.RawData = json.RawMessage(`{"diagnosticsProviders":{}}`)

	r := newReply()
	call(t, s, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{Command: "php-diagls/showConfig"}, r)
	if r.calls != 1 || r.err != nil {
		t.Errorf("showConfig should reply cleanly, got calls=%d err=%v", r.calls, r.err)
	}
	if msgs := conn.messages(); len(msgs) != 1 || !strings.Contains(msgs[0], `"diagnosticsProviders"`) {
		t.Errorf("expected the raw config in a window message, got %v", msgs)
	}

	r = newReply()
	call(t, s, protocol.MethodWorkspaceExecuteCommand, protocol.ExecuteCommandParams{Command: "php-diagls/nope"}, r)
	if r.err == nil {
		t.Error("unknown command should reply with an error")
	}
}

func TestUnknownMethodIsAcknowledged(t *testing.T) {
	s, _ := newTestServer(t)
	r := newReply()
	call(t, s, "some/unknownMethod", struct{}{}, r)
	if r.calls != 1 || r.err != nil {
		t.Errorf("expected a nil reply, got calls=%d err=%v", r.calls, r.err)
	}
}

func TestShutdownAndExit(t *testing.T) {
	s, conn := newTestServer(t)
	r := newReply()
	call(t, s, protocol.MethodShutdown, struct{}{}, r)
	if r.calls != 1 || r.err != nil {
		t.Errorf("shutdown should reply, got calls=%d err=%v", r.calls, r.err)
	}
	notify(t, s, protocol.MethodExit, struct{}{})
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if !conn.closed {
		t.Error("exit should close the connection")
	}
}

func TestHandle_MalformedParamsReturnError(t *testing.T) {
	s, _ := newTestServer(t)
	req, _ := jsonrpc2.NewNotification(protocol.MethodTextDocumentDidOpen, json.RawMessage(`{"textDocument": 42}`))
	if err := s.Handle(context.Background(), func(context.Context, interface{}, error) error { return nil }, req); err == nil {
		t.Error("expected an unmarshal error")
	}
}
