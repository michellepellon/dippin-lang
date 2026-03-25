// Package lsp implements a Language Server Protocol server for Dippin workflows.
package lsp

import (
	"sync"

	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/parser"
)

// documentStore tracks open documents and their parsed state.
type documentStore struct {
	mu   sync.RWMutex
	docs map[string]*document
}

// document represents a single open .dip file.
type document struct {
	URI     string
	Content string
	Version int32
	Parsed  *ir.Workflow
	Err     error
}

func newDocumentStore() *documentStore {
	return &documentStore{docs: make(map[string]*document)}
}

// open adds or updates a document.
func (s *documentStore) open(uri, content string, version int32) *document {
	doc := &document{URI: uri, Content: content, Version: version}
	doc.Parsed, doc.Err = parseContent(content, uri)
	s.mu.Lock()
	s.docs[uri] = doc
	s.mu.Unlock()
	return doc
}

// update replaces the content of an open document.
func (s *documentStore) update(uri, content string, version int32) *document {
	return s.open(uri, content, version)
}

// close removes a document from the store.
func (s *documentStore) close(uri string) {
	s.mu.Lock()
	delete(s.docs, uri)
	s.mu.Unlock()
}

// get returns the document for a URI, or nil if not found.
func (s *documentStore) get(uri string) *document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[uri]
}

// parseContent parses .dip content into a workflow IR.
func parseContent(content, uri string) (*ir.Workflow, error) {
	p := parser.NewParser(content, uri)
	return p.Parse()
}
