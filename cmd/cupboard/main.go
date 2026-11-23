package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `usage: cupboard <command> [arguments]

commands:
  add <shelf>       save a key and its shelf settings
  list              show all shelves
  remove <shelf>    delete a shelf and its key
  run -- <command>  start a command with the keys as env vars
  serve             start the local proxy

flags:
  --version         print the version
`

type command func(args []string, stdin io.Reader, stdout, stderr io.Writer) int

func main() {
	os.Exit(cli(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func cli(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cupboard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	showVersion := fs.Bool("version", false, "print the version")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}
	commands := map[string]command{
		"add":    cmdAdd,
		"list":   notImplemented,
		"remove": notImplemented,
		"run":    notImplemented,
		"serve":  notImplemented,
	}
	if fs.NArg() > 0 {
		if cmd, ok := commands[fs.Arg(0)]; ok {
			return cmd(fs.Args()[1:], stdin, stdout, stderr)
		}
	}
	fs.Usage()
	return 2
}

func notImplemented(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "not implemented yet")
	return 1
}
