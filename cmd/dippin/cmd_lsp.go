package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.lsp.dev/jsonrpc2"

	"github.com/2389-research/dippin-lang/lsp"
)

// CmdLSP starts the LSP server on stdio.
func (c *CLI) CmdLSP(args []string) ExitCode {
	server := lsp.NewServer()

	ctx := context.Background()
	rwc := &stdioRWC{in: os.Stdin, out: os.Stdout}
	stream := jsonrpc2.NewStream(rwc)
	conn := jsonrpc2.NewConn(stream)
	server.SetConn(conn)

	conn.Go(ctx, server.Handler())
	<-conn.Done()

	if err := conn.Err(); err != nil {
		fmt.Fprintf(c.Stderr, "lsp error: %v\n", err)
		return ExitError
	}
	return ExitOK
}

// stdioRWC wraps stdin/stdout as a ReadWriteCloser.
type stdioRWC struct {
	in  io.Reader
	out io.Writer
}

func (s *stdioRWC) Read(p []byte) (int, error)  { return s.in.Read(p) }
func (s *stdioRWC) Write(p []byte) (int, error) { return s.out.Write(p) }
func (s *stdioRWC) Close() error                { return nil }
