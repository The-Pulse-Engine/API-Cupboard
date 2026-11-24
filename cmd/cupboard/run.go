package main

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"slices"
	"syscall"

	"github.com/The-Pulse-Engine/API-Cupboard/internal/inject"
)

func cmdRun(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 2 || args[0] != "--" {
		fmt.Fprintln(stderr, "usage: cupboard run -- <command> [args...]")
		return 2
	}
	wd, err := os.Getwd()
	if err != nil {
		return fail(stderr, err)
	}
	path, err := inject.Find(wd)
	if err != nil {
		return fail(stderr, err)
	}
	vars, err := inject.Load(path)
	if err != nil {
		return fail(stderr, err)
	}
	env, err := inject.Environ(vars)
	if err != nil {
		return fail(stderr, err)
	}
	// Any cupboard.toml can ask for any shelf, for example in a cloned repo, so say what goes out.
	for _, name := range slices.Sorted(maps.Keys(vars)) {
		fmt.Fprintf(stderr, "cupboard: %s from shelf %q\n", name, vars[name])
	}

	cmd := exec.Command(args[1], args[2:]...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr

	// The console sends Ctrl+C to the child as well, so ignore it here and
	// wait for the child. SIGTERM (kill, an IDE stop button) reaches only
	// us, so pass it on, or the child keeps running with the keys.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	if err := cmd.Start(); err != nil {
		return fail(stderr, err)
	}
	go func() {
		for s := range sig {
			if s != os.Interrupt {
				cmd.Process.Signal(s) // Windows cannot send SIGTERM; the console closes the child there.
			}
		}
	}()
	return exitCode(cmd.Wait(), stderr)
}

// exitCode follows the shell rule: 128 + the signal number when the child was killed by a signal.
func exitCode(err error, stderr io.Writer) int {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		if err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return exitErr.ExitCode()
}
