// Package debug provides instrumentation and profiling tools for umd.
package debug

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"time"
)

// StartPprofServer starts a pprof HTTP server at the given address.
// Returns a stop function to gracefully shut down the server.
func StartPprofServer(addr string) (func(), error) {
	if addr == "" {
		addr = ":6060"
	}

	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(defaultTraceWriter(), "pprof server starting on %s\n", addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Give the server a moment to start and check for immediate errors
	select {
	case err := <-errCh:
		return nil, fmt.Errorf("pprof server failed: %w", err)
	case <-time.After(50 * time.Millisecond):
	}

	stop := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}

	return stop, nil
}
