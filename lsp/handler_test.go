package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// --- Test helpers ---

// captureReply returns a Replier that captures the result and error.
func captureReply(t *testing.T) (jsonrpc2.Replier, *interface{}, *error) {
	t.Helper()
	var result interface{}
	var rErr error
	reply := func(_ context.Context, r interface{}, err error) error {
		result = r
		rErr = err
		return nil
	}
	return reply, &result, &rErr
}

// mockConn records Notify calls for testing publishDiagnostics.
type mockConn struct {
	notifyCalls []mockNotify
}

type mockNotify struct {
	Method string
	Params interface{}
}

func (m *mockConn) Notify(_ context.Context, method string, params interface{}) error {
	m.notifyCalls = append(m.notifyCalls, mockNotify{Method: method, Params: params})
	return nil
}

func (m *mockConn) Call(context.Context, string, interface{}, interface{}) (jsonrpc2.ID, error) {
	return jsonrpc2.ID{}, nil
}

func (m *mockConn) Go(context.Context, jsonrpc2.Handler) {}
func (m *mockConn) Close() error                         { return nil }
func (m *mockConn) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (m *mockConn) Err() error { return nil }

// mustCall builds a jsonrpc2.Request (Call) for the given method and params.
func mustCall(t *testing.T, method string, params interface{}) jsonrpc2.Request {
	t.Helper()
	call, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(1), method, params)
	if err != nil {
		t.Fatalf("NewCall(%s): %v", method, err)
	}
	return call
}

// mustNotification builds a jsonrpc2.Request (Notification) for the given method.
func mustNotification(t *testing.T, method string, params interface{}) jsonrpc2.Request {
	t.Helper()
	n, err := jsonrpc2.NewNotification(method, params)
	if err != nil {
		t.Fatalf("NewNotification(%s): %v", method, err)
	}
	return n
}

// --- Handler dispatch tests ---

func TestHandle_Initialize(t *testing.T) {
	s := NewServer()
	reply, result, rErr := captureReply(t)

	req := mustCall(t, "initialize", protocol.InitializeParams{})
	if err := s.handle(context.Background(), reply, req); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if *rErr != nil {
		t.Fatalf("reply error: %v", *rErr)
	}

	initResult, ok := (*result).(protocol.InitializeResult)
	if !ok {
		t.Fatalf("expected InitializeResult, got %T", *result)
	}
	if initResult.Capabilities.HoverProvider != true {
		t.Error("expected hover provider enabled")
	}
	if initResult.Capabilities.DefinitionProvider != true {
		t.Error("expected definition provider enabled")
	}
	if initResult.ServerInfo == nil || initResult.ServerInfo.Name != "dippin-lsp" {
		t.Error("expected server name dippin-lsp")
	}
}

func TestHandle_NoOp_Initialized(t *testing.T) {
	s := NewServer()
	reply, result, rErr := captureReply(t)

	req := mustNotification(t, "initialized", nil)
	if err := s.handle(context.Background(), reply, req); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if *rErr != nil {
		t.Fatalf("reply error: %v", *rErr)
	}
	if *result != nil {
		t.Errorf("expected nil result for no-op, got %v", *result)
	}
}

func TestHandle_NoOp_Shutdown(t *testing.T) {
	s := NewServer()
	reply, result, rErr := captureReply(t)

	req := mustCall(t, "shutdown", nil)
	if err := s.handle(context.Background(), reply, req); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if *rErr != nil {
		t.Fatalf("reply error: %v", *rErr)
	}
	if *result != nil {
		t.Errorf("expected nil result for shutdown, got %v", *result)
	}
}

func TestHandle_UnknownMethod(t *testing.T) {
	s := NewServer()
	reply, _, rErr := captureReply(t)

	req := mustCall(t, "textDocument/unknownMethod", nil)
	// MethodNotFoundHandler replies with an error.
	_ = s.handle(context.Background(), reply, req)
	if *rErr == nil {
		t.Error("expected MethodNotFound error for unknown method")
	}
}

func TestHandle_DidOpen(t *testing.T) {
	s := NewServer()
	mc := &mockConn{}
	s.SetConn(mc)
	reply, _, rErr := captureReply(t)

	params := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     "file:///test.dip",
			Text:    testDipContent,
			Version: 1,
		},
	}
	req := mustNotification(t, "textDocument/didOpen", params)
	if err := s.handle(context.Background(), reply, req); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if *rErr != nil {
		t.Fatalf("reply error: %v", *rErr)
	}

	// Document should be in store.
	doc := s.store.get("file:///test.dip")
	if doc == nil {
		t.Fatal("expected document in store after didOpen")
	}
	if doc.Parsed == nil {
		t.Fatal("expected parsed workflow")
	}
	if doc.Parsed.Name != "Test" {
		t.Errorf("expected workflow name Test, got %s", doc.Parsed.Name)
	}

	// Diagnostics should have been published via mockConn.
	if len(mc.notifyCalls) == 0 {
		t.Error("expected diagnostics to be published")
	} else if mc.notifyCalls[0].Method != "textDocument/publishDiagnostics" {
		t.Errorf("expected publishDiagnostics, got %s", mc.notifyCalls[0].Method)
	}
}

func TestHandle_DidOpen_InvalidJSON(t *testing.T) {
	s := NewServer()
	reply, _, rErr := captureReply(t)

	// Send params that don't match DidOpenTextDocumentParams structure.
	// Use a struct that marshals to valid JSON but can't unmarshal to the expected type.
	call, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(1), "textDocument/didOpen", map[string]int{"textDocument": 42})
	if err != nil {
		t.Fatalf("NewCall: %v", err)
	}
	_ = s.handle(context.Background(), reply, call)
	if *rErr == nil {
		t.Error("expected error reply for invalid params")
	}
}

func TestHandle_DidChange(t *testing.T) {
	s := NewServer()
	mc := &mockConn{}
	s.SetConn(mc)
	reply, _, _ := captureReply(t)

	// First open a document.
	openParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     "file:///test.dip",
			Text:    testDipContent,
			Version: 1,
		},
	}
	openReq := mustNotification(t, "textDocument/didOpen", openParams)
	_ = s.handle(context.Background(), reply, openReq)

	// Now change it.
	updatedContent := `workflow Updated
  goal: "Updated"
  start: A
  exit: B

  agent A
    prompt:
      Do A.

  agent B
    prompt:
      Do B.

  edges
    A -> B
`
	changeParams := protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: "file:///test.dip"},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{Text: updatedContent},
		},
	}
	reply2, _, rErr2 := captureReply(t)
	changeReq := mustNotification(t, "textDocument/didChange", changeParams)
	if err := s.handle(context.Background(), reply2, changeReq); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if *rErr2 != nil {
		t.Fatalf("reply error: %v", *rErr2)
	}

	doc := s.store.get("file:///test.dip")
	if doc == nil {
		t.Fatal("expected document in store after didChange")
	}
	if doc.Parsed == nil || doc.Parsed.Name != "Updated" {
		t.Errorf("expected workflow name Updated, got %v", doc.Parsed)
	}
}

func TestHandle_DidClose(t *testing.T) {
	s := NewServer()
	reply, _, _ := captureReply(t)

	// Open a document first.
	openParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  "file:///test.dip",
			Text: testDipContent,
		},
	}
	openReq := mustNotification(t, "textDocument/didOpen", openParams)
	_ = s.handle(context.Background(), reply, openReq)

	// Close it.
	closeParams := protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.dip"},
	}
	reply2, _, rErr2 := captureReply(t)
	closeReq := mustNotification(t, "textDocument/didClose", closeParams)
	if err := s.handle(context.Background(), reply2, closeReq); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if *rErr2 != nil {
		t.Fatalf("reply error: %v", *rErr2)
	}

	if s.store.get("file:///test.dip") != nil {
		t.Error("expected document removed from store after didClose")
	}
}

func TestHandle_Hover_Found(t *testing.T) {
	s := NewServer()
	reply, _, _ := captureReply(t)

	// Open document.
	openParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  "file:///test.dip",
			Text: testDipContent,
		},
	}
	openReq := mustNotification(t, "textDocument/didOpen", openParams)
	_ = s.handle(context.Background(), reply, openReq)

	// Find a node line — "human Ask" should be around line 6 (0-indexed).
	doc := s.store.get("file:///test.dip")
	if doc == nil || doc.Parsed == nil {
		t.Fatal("document not parsed")
	}

	// Find the Ask node's line.
	var askLine uint32
	for _, n := range doc.Parsed.Nodes {
		if n.ID == "Ask" && n.Source.Line > 0 {
			askLine = uint32(n.Source.Line - 1) // 0-indexed
			break
		}
	}

	hoverParams := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.dip"},
			Position:     protocol.Position{Line: askLine, Character: 2},
		},
	}
	reply2, result2, rErr2 := captureReply(t)
	hoverReq := mustCall(t, "textDocument/hover", hoverParams)
	if err := s.handle(context.Background(), reply2, hoverReq); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if *rErr2 != nil {
		t.Fatalf("reply error: %v", *rErr2)
	}
	if *result2 == nil {
		t.Fatal("expected non-nil hover result at node line")
	}

	hover, ok := (*result2).(protocol.Hover)
	if !ok {
		t.Fatalf("expected Hover, got %T", *result2)
	}
	if hover.Contents.Value == "" {
		t.Error("expected non-empty hover content")
	}
}

func TestHandle_Hover_NotFound(t *testing.T) {
	s := NewServer()
	reply, _, _ := captureReply(t)

	openParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  "file:///test.dip",
			Text: testDipContent,
		},
	}
	openReq := mustNotification(t, "textDocument/didOpen", openParams)
	_ = s.handle(context.Background(), reply, openReq)

	// Line 0 is the workflow header, no node there.
	hoverParams := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.dip"},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	}
	reply2, result2, _ := captureReply(t)
	hoverReq := mustCall(t, "textDocument/hover", hoverParams)
	_ = s.handle(context.Background(), reply2, hoverReq)

	if *result2 != nil {
		t.Errorf("expected nil result at non-node line, got %v", *result2)
	}
}

func TestHandle_Hover_MissingDoc(t *testing.T) {
	s := NewServer()
	reply, result, _ := captureReply(t)

	hoverParams := protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///missing.dip"},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	}
	req := mustCall(t, "textDocument/hover", hoverParams)
	_ = s.handle(context.Background(), reply, req)

	if *result != nil {
		t.Errorf("expected nil for missing document, got %v", *result)
	}
}

func TestHandle_Completion(t *testing.T) {
	s := NewServer()
	reply, _, _ := captureReply(t)

	openParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  "file:///test.dip",
			Text: testDipContent,
		},
	}
	openReq := mustNotification(t, "textDocument/didOpen", openParams)
	_ = s.handle(context.Background(), reply, openReq)

	compParams := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.dip"},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	}
	reply2, result2, rErr2 := captureReply(t)
	compReq := mustCall(t, "textDocument/completion", compParams)
	if err := s.handle(context.Background(), reply2, compReq); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if *rErr2 != nil {
		t.Fatalf("reply error: %v", *rErr2)
	}

	items, ok := (*result2).([]protocol.CompletionItem)
	if !ok {
		t.Fatalf("expected []CompletionItem, got %T", *result2)
	}
	// Should include node IDs (Ask, Done) and field completions.
	hasNodeID := false
	for _, item := range items {
		if item.Label == "Ask" || item.Label == "Done" {
			hasNodeID = true
			break
		}
	}
	if !hasNodeID {
		t.Error("expected node ID in completion items")
	}
}

func TestHandle_Completion_MissingDoc(t *testing.T) {
	s := NewServer()
	reply, result, _ := captureReply(t)

	compParams := protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///missing.dip"},
		},
	}
	req := mustCall(t, "textDocument/completion", compParams)
	_ = s.handle(context.Background(), reply, req)

	if *result != nil {
		t.Errorf("expected nil for missing document, got %v", *result)
	}
}

func TestHandle_Definition_Found(t *testing.T) {
	s := NewServer()
	reply, _, _ := captureReply(t)

	openParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  "file:///test.dip",
			Text: testDipContent,
		},
	}
	openReq := mustNotification(t, "textDocument/didOpen", openParams)
	_ = s.handle(context.Background(), reply, openReq)

	doc := s.store.get("file:///test.dip")
	if doc == nil || doc.Parsed == nil {
		t.Fatal("document not parsed")
	}

	// Find the edges section to get a line with "Done" referenced.
	// The edges section has "Ask -> Done", so find "Done" on that line.
	var edgeLine uint32
	for _, e := range doc.Parsed.Edges {
		if e.Source.Line > 0 {
			edgeLine = uint32(e.Source.Line - 1)
			break
		}
	}

	// Position cursor on "Done" in "Ask -> Done" — find its column.
	lines := splitLines(doc.Content)
	var doneCol uint32
	if int(edgeLine) < len(lines) {
		for i := 0; i+3 < len(lines[edgeLine]); i++ {
			if lines[edgeLine][i:i+4] == "Done" {
				doneCol = uint32(i)
				break
			}
		}
	}

	defParams := protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.dip"},
			Position:     protocol.Position{Line: edgeLine, Character: doneCol},
		},
	}
	reply2, result2, rErr2 := captureReply(t)
	defReq := mustCall(t, "textDocument/definition", defParams)
	if err := s.handle(context.Background(), reply2, defReq); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if *rErr2 != nil {
		t.Fatalf("reply error: %v", *rErr2)
	}
	if *result2 == nil {
		t.Fatal("expected non-nil definition result")
	}
}

// splitLines is a test helper that splits content into lines.
func splitLines(content string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, content[start:i])
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, content[start:])
	}
	return lines
}

func TestHandle_Definition_NotFound(t *testing.T) {
	s := NewServer()
	reply, _, _ := captureReply(t)

	openParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  "file:///test.dip",
			Text: testDipContent,
		},
	}
	openReq := mustNotification(t, "textDocument/didOpen", openParams)
	_ = s.handle(context.Background(), reply, openReq)

	// Position on "workflow" keyword — not a node ID.
	defParams := protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.dip"},
			Position:     protocol.Position{Line: 0, Character: 0},
		},
	}
	reply2, result2, _ := captureReply(t)
	defReq := mustCall(t, "textDocument/definition", defParams)
	_ = s.handle(context.Background(), reply2, defReq)

	// "workflow" is not a node ID → resolveDefinition returns nil.
	// But the reply wraps it, so result could be a nil *protocol.Location.
	if *result2 != nil {
		loc, ok := (*result2).(*protocol.Location)
		if ok && loc != nil {
			t.Errorf("expected nil location for non-node word, got %v", loc)
		}
	}
}

func TestHandle_DocumentSymbol(t *testing.T) {
	s := NewServer()
	reply, _, _ := captureReply(t)

	openParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  "file:///test.dip",
			Text: testDipContent,
		},
	}
	openReq := mustNotification(t, "textDocument/didOpen", openParams)
	_ = s.handle(context.Background(), reply, openReq)

	symParams := protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///test.dip"},
	}
	reply2, result2, rErr2 := captureReply(t)
	symReq := mustCall(t, "textDocument/documentSymbol", symParams)
	if err := s.handle(context.Background(), reply2, symReq); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if *rErr2 != nil {
		t.Fatalf("reply error: %v", *rErr2)
	}

	symbols, ok := (*result2).([]protocol.DocumentSymbol)
	if !ok {
		t.Fatalf("expected []DocumentSymbol, got %T", *result2)
	}
	if len(symbols) == 0 {
		t.Error("expected at least one symbol")
	}
}

func TestHandle_DocumentSymbol_MissingDoc(t *testing.T) {
	s := NewServer()
	reply, result, _ := captureReply(t)

	symParams := protocol.DocumentSymbolParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: "file:///missing.dip"},
	}
	req := mustCall(t, "textDocument/documentSymbol", symParams)
	_ = s.handle(context.Background(), reply, req)

	if *result != nil {
		t.Errorf("expected nil for missing document, got %v", *result)
	}
}

func TestPublishDiagnostics_NilConn(t *testing.T) {
	s := NewServer()
	// conn is nil by default — should not panic.
	doc := &document{URI: "file:///test.dip", Parsed: nil}
	s.publishDiagnostics(context.Background(), doc)
	// No panic = pass.
}

func TestConvertDiagnostic_ViaHandler(t *testing.T) {
	// Verify the full diagnostic conversion path by opening a document
	// with a known lint issue and checking the published diagnostics.
	s := NewServer()
	mc := &mockConn{}
	s.SetConn(mc)
	reply, _, _ := captureReply(t)

	// Open a minimal valid document — collectDiagnostics runs validate + lint.
	openParams := protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:     "file:///diag_test.dip",
			Text:    testDipContent,
			Version: 1,
		},
	}
	req := mustNotification(t, "textDocument/didOpen", openParams)
	_ = s.handle(context.Background(), reply, req)

	// Diagnostics should have been published (may be empty for valid file).
	if len(mc.notifyCalls) == 0 {
		t.Fatal("expected at least one Notify call for diagnostics")
	}
	if mc.notifyCalls[0].Method != "textDocument/publishDiagnostics" {
		t.Errorf("expected publishDiagnostics method, got %s", mc.notifyCalls[0].Method)
	}
}

func TestSetConn(t *testing.T) {
	s := NewServer()
	mc := &mockConn{}
	s.SetConn(mc)

	if s.conn != mc {
		t.Error("expected conn to be set")
	}
}

func TestHandler_ReturnsNonNil(t *testing.T) {
	s := NewServer()
	h := s.Handler()
	if h == nil {
		t.Fatal("expected non-nil handler from Handler()")
	}
}

func TestHandler_DispatchesRequests(t *testing.T) {
	s := NewServer()
	h := s.Handler()
	reply, result, rErr := captureReply(t)

	req := mustCall(t, "initialize", protocol.InitializeParams{})
	if err := h(context.Background(), reply, req); err != nil {
		t.Fatalf("handler dispatch: %v", err)
	}
	if *rErr != nil {
		t.Fatalf("reply error: %v", *rErr)
	}
	if *result == nil {
		t.Fatal("expected non-nil result from initialize via Handler()")
	}
}
