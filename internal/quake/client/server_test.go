package client

import (
	"net"
	"net/http"
	"testing"

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
	defer l.Close()

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
	defer udpConn.Close()

	proxy, err := NewProxy(udpConn.LocalAddr().String(), logger)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	if proxy == nil {
		t.Error("expected proxy to be non-nil")
	}
}
