package main

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestServeUsage(t *testing.T) {
	for _, args := range [][]string{{"serve", "--port", "0"}, {"serve", "--port", "70000"}, {"serve", "extra"}} {
		if _, stderr, code := runCLI(t, "", args...); code != 2 || !strings.Contains(stderr, "usage") {
			t.Errorf("%v: code %d, stderr %q", args, code, stderr)
		}
	}
}

func TestServePortInUse(t *testing.T) {
	setup(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := fmt.Sprint(l.Addr().(*net.TCPAddr).Port)
	_, stderr, code := runCLI(t, "", "serve", "--port", port)
	if code != 1 || !strings.Contains(stderr, "another cupboard serve") {
		t.Fatalf("code %d, stderr %q", code, stderr)
	}
}
