// Package proxy serves the local proxy on 127.0.0.1.
package proxy

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Listen opens the port on 127.0.0.1 only, never on all interfaces (security requirement 2).
func Listen(port int) (net.Listener, error) {
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("cannot listen on 127.0.0.1:%d (is another cupboard serve running?): %w", port, err)
	}
	return l, nil
}

// NewServer returns the HTTP server for l. Requests that pass the Host check go to h.
func NewServer(l net.Listener, h http.Handler) *http.Server {
	return &http.Server{
		Handler: checkHost(l.Addr().(*net.TCPAddr).Port, h),
		// Only a header timeout: a read or write timeout would cut off streams and uploads.
		ReadHeaderTimeout: 10 * time.Second,
	}
}

// checkHost rejects every request for another Host, which stops DNS rebinding
// from a browser (security requirement 3).
func checkHost(port int, next http.Handler) http.Handler {
	allowed := map[string]bool{
		"127.0.0.1:" + strconv.Itoa(port): true,
		"localhost:" + strconv.Itoa(port): true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed[strings.ToLower(r.Host)] {
			http.Error(w, fmt.Sprintf("cupboard: unknown Host, use 127.0.0.1:%d or localhost:%d", port, port), http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}
