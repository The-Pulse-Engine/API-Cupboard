package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/The-Pulse-Engine/API-Cupboard/internal/store"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

const addUsage = "usage: cupboard add <shelf> [--upstream URL] [--inject bearer|basic|header|query] [--header NAME] [--query NAME] [--stdin] [--force]"

func cmdAdd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, addUsage)
		fs.PrintDefaults()
	}
	upstream := fs.String("upstream", "", "upstream URL, https only (asked if not set)")
	inject := fs.String("inject", "bearer", "how the proxy adds the key: bearer, basic, header or query")
	header := fs.String("header", "", "header name, for --inject header")
	query := fs.String("query", "", "query parameter name, for --inject query")
	fromStdin := fs.Bool("stdin", false, "read the key from stdin, for scripts")
	force := fs.Bool("force", false, "replace the shelf if it exists")

	// flag stops at the first non-flag argument, so parse again after the shelf name.
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}
	name := fs.Arg(0)
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return 2
	}

	path, shelves, err := loadShelves()
	if err != nil {
		return fail(stderr, err)
	}
	idx := slices.IndexFunc(shelves, func(s store.Shelf) bool { return s.Name == name })
	if idx >= 0 && !*force {
		return fail(stderr, fmt.Errorf("shelf %q already exists, use --force to replace it", name))
	}

	in := bufio.NewReader(stdin)
	if *upstream == "" {
		if *fromStdin {
			return fail(stderr, errors.New("--stdin needs --upstream, because stdin holds the key"))
		}
		fmt.Fprint(stderr, "Upstream URL: ")
		if *upstream, err = readLine(in); err != nil {
			return fail(stderr, err)
		}
	}
	shelf := store.Shelf{Name: name, Upstream: *upstream, Inject: *inject, Header: *header, Query: *query}
	if err := shelf.Validate(); err != nil {
		return fail(stderr, err)
	}

	key, err := readKey(stdin, in, *fromStdin, stderr)
	if err != nil {
		return fail(stderr, err)
	}
	if key == "" {
		return fail(stderr, errors.New("the key is empty"))
	}

	oldKey, oldErr := store.ReadKey(name)
	if err := store.SaveKey(name, key); err != nil {
		return fail(stderr, err)
	}
	if idx >= 0 {
		shelves[idx] = shelf
	} else {
		shelves = append(shelves, shelf)
	}
	if err := store.SaveShelves(path, shelves); err != nil {
		// Put the keychain back as it was, so it matches shelves.json.
		if oldErr == nil {
			store.SaveKey(name, oldKey)
		} else {
			store.DeleteKey(name)
		}
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "added shelf %s\n", name)
	return 0
}

// readKey reads the key without showing it when stdin is a terminal.
func readKey(stdin io.Reader, in *bufio.Reader, all bool, stderr io.Writer) (string, error) {
	if all {
		data, err := io.ReadAll(in)
		return strings.TrimRight(string(data), "\r\n"), err
	}
	fmt.Fprint(stderr, "Key: ")
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		data, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stderr)
		return string(data), err
	}
	return readLine(in)
}

func readLine(in *bufio.Reader) (string, error) {
	line, err := in.ReadString('\n')
	if err == io.EOF && line != "" {
		err = nil
	}
	return strings.TrimRight(line, "\r\n"), err
}

func loadShelves() (string, []store.Shelf, error) {
	path, err := store.ShelvesPath()
	if err != nil {
		return "", nil, err
	}
	shelves, err := store.LoadShelves(path)
	return path, shelves, err
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "cupboard: %v\n", err)
	return 1
}

func cmdList(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: cupboard list")
		return 2
	}
	_, shelves, err := loadShelves()
	if err != nil {
		return fail(stderr, err)
	}
	if len(shelves) == 0 {
		fmt.Fprintln(stdout, "No shelves yet. Run: cupboard add <shelf>")
		return 0
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tUPSTREAM\tINJECT")
	for _, s := range shelves {
		rule := s.Inject
		if s.Inject == "header" {
			rule += " " + s.Header
		} else if s.Inject == "query" {
			rule += " " + s.Query
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Upstream, rule)
	}
	w.Flush()
	return 0
}

func cmdRemove(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: cupboard remove <shelf>")
		return 2
	}
	name := args[0]
	path, shelves, err := loadShelves()
	if err != nil {
		return fail(stderr, err)
	}
	idx := slices.IndexFunc(shelves, func(s store.Shelf) bool { return s.Name == name })
	if idx < 0 {
		return fail(stderr, fmt.Errorf("shelf %q does not exist, run cupboard list to see your shelves", name))
	}
	// A key that is already gone is fine: the shelf still goes.
	if err := store.DeleteKey(name); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fail(stderr, err)
	}
	if err := store.SaveShelves(path, slices.Delete(shelves, idx, idx+1)); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "removed shelf %s\n", name)
	return 0
}
