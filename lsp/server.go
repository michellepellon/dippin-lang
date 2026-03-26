package lsp

import (
	"context"
	"encoding/json"
	"sync"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// Server is the Dippin LSP server.
type Server struct {
	mu    sync.RWMutex
	conn  jsonrpc2.Conn
	store *documentStore
}

// NewServer creates a new LSP server.
func NewServer() *Server {
	return &Server{store: newDocumentStore()}
}

// methodHandler is a function that handles a specific LSP method.
type methodHandler func(context.Context, jsonrpc2.Replier, jsonrpc2.Request) error

// Handler returns the jsonrpc2 handler for the server.
func (s *Server) Handler() jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		return s.handle(ctx, reply, req)
	}
}

// methodTable returns the dispatch table for all supported methods.
func (s *Server) methodTable() map[string]methodHandler {
	return map[string]methodHandler{
		"initialize":                  s.handleInitialize,
		"textDocument/didOpen":        s.handleDidOpen,
		"textDocument/didChange":      s.handleDidChange,
		"textDocument/didClose":       s.handleDidClose,
		"textDocument/hover":          s.handleHover,
		"textDocument/definition":     s.handleDefinition,
		"textDocument/completion":     s.handleCompletion,
		"textDocument/documentSymbol": s.handleDocumentSymbol,
	}
}

// handle dispatches incoming requests to the appropriate method handler.
func (s *Server) handle(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	if isNoOp(req.Method()) {
		return reply(ctx, nil, nil)
	}
	if handler, ok := s.methodTable()[req.Method()]; ok {
		return handler(ctx, reply, req)
	}
	return jsonrpc2.MethodNotFoundHandler(ctx, reply, req)
}

// isNoOp returns true for methods that need only an empty ack.
func isNoOp(method string) bool {
	return method == "initialized" || method == "shutdown" || method == "exit"
}

// handleInitialize responds to the initialize request with server capabilities.
func (s *Server) handleInitialize(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	result := protocol.InitializeResult{
		Capabilities: serverCapabilities(),
		ServerInfo: &protocol.ServerInfo{
			Name:    "dippin-lsp",
			Version: "0.1.0",
		},
	}
	return reply(ctx, result, nil)
}

// serverCapabilities returns the LSP capabilities this server supports.
func serverCapabilities() protocol.ServerCapabilities {
	return protocol.ServerCapabilities{
		TextDocumentSync:   protocol.TextDocumentSyncKindFull,
		HoverProvider:      true,
		DefinitionProvider: true,
		CompletionProvider: &protocol.CompletionOptions{
			TriggerCharacters: []string{".", ">"},
		},
		DocumentSymbolProvider: true,
	}
}

// handleDidOpen processes textDocument/didOpen notifications.
func (s *Server) handleDidOpen(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidOpenTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	doc := s.store.open(string(params.TextDocument.URI), params.TextDocument.Text, params.TextDocument.Version)
	s.publishDiagnostics(ctx, doc)
	return reply(ctx, nil, nil)
}

// handleDidChange processes textDocument/didChange notifications.
func (s *Server) handleDidChange(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidChangeTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	if len(params.ContentChanges) == 0 {
		return reply(ctx, nil, nil)
	}
	content := params.ContentChanges[len(params.ContentChanges)-1].Text
	doc := s.store.update(string(params.TextDocument.URI), content, params.TextDocument.Version)
	s.publishDiagnostics(ctx, doc)
	return reply(ctx, nil, nil)
}

// handleDidClose processes textDocument/didClose notifications.
func (s *Server) handleDidClose(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	var params protocol.DidCloseTextDocumentParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}
	s.store.close(string(params.TextDocument.URI))
	return reply(ctx, nil, nil)
}

// SetConn sets the connection for sending notifications.
func (s *Server) SetConn(conn jsonrpc2.Conn) {
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
}

// getConn returns the connection, safe for concurrent access.
func (s *Server) getConn() jsonrpc2.Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conn
}
