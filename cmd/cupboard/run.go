package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"

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

	cmd := exec.Command(args[1], args[2:]...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr

	// The console sends Ctrl+C to the child as well. Ignore it here and
	// wait, so the child can shut down and we return its exit code.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)

	err = cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	if err != nil {
		return fail(stderr, err)
	}
	return 0
}
