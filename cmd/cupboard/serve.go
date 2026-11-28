package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/The-Pulse-Engine/API-Cupboard/internal/proxy"
	"github.com/The-Pulse-Engine/API-Cupboard/internal/store"
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
	path, err := store.ShelvesPath()
	if err != nil {
		return fail(stderr, err)
	}
	load := func() ([]store.Shelf, error) { return store.LoadShelves(path) }

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	l, err := proxy.Listen(*port)
	if err != nil {
		return fail(stderr, err)
	}
	srv := proxy.NewServer(l, proxy.Handler(load, nil))
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(l) }()
	fmt.Fprintf(stdout, "cupboard: proxy on http://127.0.0.1:%d, press Ctrl+C to stop\n", *port)

	select {
	case err := <-errc:
		return fail(stderr, err)
	case <-ctx.Done():
	}
	stop() // from here, a second Ctrl+C ends the process at once

	// Give open requests a few seconds to finish, then close the rest (an open stream, for example).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownWait)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); errors.Is(err, context.DeadlineExceeded) {
		srv.Close()
		fmt.Fprintln(stdout, "cupboard: closed requests that were still open")
	} else if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintln(stdout, "cupboard: proxy stopped")
	return 0
}

// shutdownWait is how long open requests get to finish. Tests make it short.
var shutdownWait = 5 * time.Second
