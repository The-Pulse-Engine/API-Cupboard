package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/The-Pulse-Engine/API-Cupboard/internal/proxy"
)

func cmdServe(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	port := fs.Int("port", 9090, "port on 127.0.0.1")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || *port < 1 || *port > 65535 {
		fmt.Fprintln(stderr, "usage: cupboard serve [--port N], with N from 1 to 65535")
		return 2
	}

	// Register before the port opens, so a stop signal that comes right after cannot kill us mid-start.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	l, err := proxy.Listen(*port)
	if err != nil {
		return fail(stderr, err)
	}
	srv := proxy.NewServer(l, http.NotFoundHandler())
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(l) }()
	fmt.Fprintf(stdout, "cupboard: proxy on http://127.0.0.1:%d, press Ctrl+C to stop\n", *port)

	select {
	case err := <-errc:
		return fail(stderr, err)
	case <-ctx.Done():
	}
	// Give open requests a few seconds to finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, "cupboard: proxy stopped")
	return 0
}
