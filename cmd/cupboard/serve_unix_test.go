//go:build !windows

package main

import (
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestServeStopsCleanlyOnSignal(t *testing.T) {
	setup(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	done := make(chan int, 1)
	go func() {
		done <- cli([]string{"serve", "--port", fmt.Sprint(port)}, strings.NewReader(""), io.Discard, io.Discard)
	}()
	// Wait until the port answers; serve registered for signals before it opened the port.
	for start := time.Now(); ; time.Sleep(20 * time.Millisecond) {
		if c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
			c.Close()
			break
		}
		if time.Since(start) > 5*time.Second {
			t.Fatal("serve did not open the port")
		}
	}
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("code %d, want 0", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop")
	}
}
