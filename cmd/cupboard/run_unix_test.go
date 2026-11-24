//go:build !windows

package main

import (
	"bufio"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunKilledBySignal(t *testing.T) {
	setupProject(t)
	_, stderr, code := runCLI(t, "", "run", "--", buildHelper(t), "CUPBOARD_TEST_KEY", "kill")
	if code != 128+int(syscall.SIGKILL) {
		t.Fatalf("code %d, want %d, stderr %q", code, 128+int(syscall.SIGKILL), stderr)
	}
}

func TestRunForwardsSIGTERM(t *testing.T) {
	setupProject(t)
	helper := buildHelper(t)
	r, w := io.Pipe()
	done := make(chan int, 1)
	go func() {
		code := cli([]string{"run", "--", helper, "CUPBOARD_TEST_KEY", "wait"}, strings.NewReader(""), w, io.Discard)
		w.Close()
		done <- code
	}()
	// The child is running once it prints, and run has registered for signals before it started.
	if line, err := bufio.NewReader(r).ReadString('\n'); err != nil || line != "sk-run ready\n" {
		t.Fatalf("first line %q, %v", line, err)
	}
	go io.Copy(io.Discard, r)
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != 128+int(syscall.SIGTERM) {
			t.Fatalf("code %d, want %d", code, 128+int(syscall.SIGTERM))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the child still runs after SIGTERM")
	}
}
