package main

import (
	"bytes"
	"strings"
	"testing"
)

func runCLI(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = cli(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"nope"}, {"--bad-flag"}} {
		_, stderr, code := runCLI(t, "", args...)
		if code != 2 || !strings.Contains(stderr, "usage: cupboard") {
			t.Errorf("%v: code %d, stderr %q", args, code, stderr)
		}
	}
}

func TestVersion(t *testing.T) {
	stdout, _, code := runCLI(t, "", "--version")
	if code != 0 || stdout != "dev\n" {
		t.Fatalf("code %d, stdout %q", code, stdout)
	}
}
