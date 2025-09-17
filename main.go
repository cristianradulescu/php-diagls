package main

import (
	"context"
	"flag"
	"io"
	"log"
	"os"

	"github.com/cristianradulescu/php-diagls/internal/logging"
	"github.com/cristianradulescu/php-diagls/internal/server"
	"go.lsp.dev/jsonrpc2"
)

func main() {
	var stdin bool

	flag.BoolVar(&stdin, "stdin", false, "Use stdin/stdout for communication")
	flag.Parse()

	if stdin {
		log.SetOutput(os.Stderr)

	}
	log.Printf("%s%s Starting PHP Diagnostics LSP server", logging.LogTagLSP, logging.LogTagMain)

	stream := jsonrpc2.NewStream(struct {
		io.Reader
		io.Writer
		io.Closer
	}{
		os.Stdin,  // Read from standard input.
		os.Stdout, // Write to standard output.
		os.Stdin,  // Close standard input (though typically stdin isn't closed by the server).
	})

	ctx := context.Background()
	conn := jsonrpc2.NewConn(stream)
	log.Printf("%s%s LSP server connection established", logging.LogTagLSP, logging.LogTagMain)

	lspServer := server.New(conn)
	log.Printf("%s%s Starting to handle requests...", logging.LogTagLSP, logging.LogTagMain)
	conn.Go(ctx, lspServer.Handle)

	// Wait for the connection to be done (e.g., closed by the client or an error occurs).
	log.Printf("%s%s LSP server is running, waiting for requests...", logging.LogTagLSP, logging.LogTagMain)
	<-conn.Done()

	// Check for any errors that occurred during the connection's lifetime.
	if err := conn.Err(); err != nil {
		log.Fatalf("%s%s LSP server stopped with error: %v", logging.LogTagLSP, logging.LogTagMain, err)
	}

	log.Printf("%s%s LSP server shutdown complete", logging.LogTagLSP, logging.LogTagMain)
}
