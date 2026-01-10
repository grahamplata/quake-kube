package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
)

type Server struct {
	Addr       string
	Handler    http.Handler
	ServerAddr string
	Logger     *zap.Logger
}

// Serve starts the multiplexed server on the provided listener.
// It routes WebSocket upgrade requests to the UDP proxy and all other
// HTTP requests to the configured handler.
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	m := cmux.New(l)
	websocketL := m.Match(cmux.HTTP1HeaderField("Upgrade", "websocket"))
	httpL := m.Match(cmux.Any())

	// Error channel to collect errors from child goroutines
	errChan := make(chan error, 2)

	// HTTP server for regular requests
	httpServer := &http.Server{
		Addr:           s.Addr,
		Handler:        s.Handler,
		ReadTimeout:    5 * time.Minute,
		WriteTimeout:   5 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		if err := httpServer.Serve(httpL); err != nil && err != cmux.ErrListenerClosed && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("http server: %w", err)
		}
	}()

	// WebSocket proxy setup
	host, port, err := net.SplitHostPort(s.ServerAddr)
	if err != nil {
		return err
	}
	proxyTarget := s.ServerAddr
	if net.ParseIP(host).IsUnspecified() {
		// handle case where host is 0.0.0.0
		proxyTarget = net.JoinHostPort("127.0.0.1", port)
	}
	wsproxy, err := NewProxy(proxyTarget, s.Logger)
	if err != nil {
		return err
	}

	// WebSocket server
	wsServer := &http.Server{
		Handler: wsproxy,
	}

	go func() {
		if err := wsServer.Serve(websocketL); err != nil && err != cmux.ErrListenerClosed && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("websocket server: %w", err)
		}
	}()

	// Start cmux in a goroutine so we can handle shutdown
	go func() {
		if err := m.Serve(); err != nil && err != cmux.ErrListenerClosed {
			errChan <- fmt.Errorf("cmux: %w", err)
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		s.Logger.Info("shutting down servers")
		// Graceful shutdown with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Shutdown both servers gracefully
		var shutdownErr error
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			shutdownErr = fmt.Errorf("http server shutdown: %w", err)
		}
		if err := wsServer.Shutdown(shutdownCtx); err != nil {
			if shutdownErr != nil {
				shutdownErr = fmt.Errorf("%v; websocket server shutdown: %w", shutdownErr, err)
			} else {
				shutdownErr = fmt.Errorf("websocket server shutdown: %w", err)
			}
		}
		// Close the listener to stop cmux
		l.Close()
		return shutdownErr
	case err := <-errChan:
		return err
	}
}

// ListenAndServe creates a TCP listener and starts the server.
func (s *Server) ListenAndServe(ctx context.Context) error {
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	return s.Serve(ctx, l)
}
