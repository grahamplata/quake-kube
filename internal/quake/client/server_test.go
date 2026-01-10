package client

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
)

// TestServerStruct verifies the Server struct can be created correctly
func TestServerStruct(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logger := zap.NewNop()

	s := &Server{
		Addr:       "127.0.0.1:8080",
		Handler:    handler,
		ServerAddr: "127.0.0.1:27960",
		Logger:     logger,
	}

	if s.Addr != "127.0.0.1:8080" {
		t.Errorf("expected Addr 127.0.0.1:8080, got %s", s.Addr)
	}
	if s.ServerAddr != "127.0.0.1:27960" {
		t.Errorf("expected ServerAddr 127.0.0.1:27960, got %s", s.ServerAddr)
	}
	if s.Handler == nil {
		t.Error("expected Handler to be set")
	}
	if s.Logger == nil {
		t.Error("expected Logger to be set")
	}
}

// TestCmuxImport verifies the soheilhy/cmux package is correctly imported and usable.
func TestCmuxImport(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			t.Errorf("failed to close listener: %v", err)
		}
	}()

	m := cmux.New(l)

	wsL := m.Match(cmux.HTTP1HeaderField("Upgrade", "websocket"))
	httpL := m.Match(cmux.Any())

	if wsL == nil {
		t.Error("WebSocket listener is nil")
	}
	if httpL == nil {
		t.Error("HTTP listener is nil")
	}

	_ = cmux.ErrListenerClosed
}

// TestNewProxy verifies the WebSocket proxy can be created
func TestNewProxy(t *testing.T) {
	logger := zap.NewNop()

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to resolve UDP addr: %v", err)
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatalf("failed to create UDP listener: %v", err)
	}
	defer func() {
		if err := udpConn.Close(); err != nil {
			t.Errorf("failed to close UDP listener: %v", err)
		}
	}()

	proxy, err := NewProxy(udpConn.LocalAddr().String(), logger)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	if proxy == nil {
		t.Error("expected proxy to be non-nil")
	}
}

// TestServer_HTTPRouting tests that HTTP requests are routed correctly through cmux
func TestServer_HTTPRouting(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("http-ok"))
		if err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	})

	// Create a UDP listener for the proxy target
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to resolve UDP addr: %v", err)
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatalf("failed to create UDP listener: %v", err)
	}
	defer func() {
		if err := udpConn.Close(); err != nil {
			t.Errorf("failed to close UDP listener: %v", err)
		}
	}()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	logger := zap.NewNop()
	s := &Server{
		Addr:       l.Addr().String(),
		Handler:    handler,
		ServerAddr: udpConn.LocalAddr().String(),
		Logger:     logger,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- s.Serve(ctx, l)
	}()

	// Give the server time to start
	time.Sleep(50 * time.Millisecond)

	// Make HTTP request
	resp, err := http.Get("http://" + l.Addr().String() + "/test")
	if err != nil {
		t.Fatalf("failed to make HTTP request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "http-ok" {
		t.Errorf("expected body 'http-ok', got %q", string(body))
	}

	// Trigger graceful shutdown
	cancel()

	// Wait for server to stop
	select {
	case <-errChan:
		// Server stopped, no error expected on graceful shutdown
	case <-time.After(5 * time.Second):
		t.Error("server did not shut down in time")
	}
}

// TestServer_GracefulShutdown tests that the server shuts down gracefully on context cancellation
func TestServer_GracefulShutdown(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	udpAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	udpConn, _ := net.ListenUDP("udp", udpAddr)
	defer func() {
		if err := udpConn.Close(); err != nil {
			t.Errorf("failed to close UDP listener: %v", err)
		}
	}()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	logger := zap.NewNop()
	s := &Server{
		Addr:       l.Addr().String(),
		Handler:    handler,
		ServerAddr: udpConn.LocalAddr().String(),
		Logger:     logger,
	}

	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error, 1)
	go func() {
		errChan <- s.Serve(ctx, l)
	}()

	time.Sleep(50 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Server should shut down within reasonable time
	select {
	case err := <-errChan:
		// Graceful shutdown should not return an error (or nil)
		if err != nil {
			t.Logf("shutdown returned: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Error("server did not shut down within timeout")
	}
}
