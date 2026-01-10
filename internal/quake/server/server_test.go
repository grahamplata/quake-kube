package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	quakenet "github.com/grahamplata/quake-kube/internal/quake/net"
	"go.uber.org/zap"
)

type mockNetClient struct {
	status *quakenet.StatusResponse
	err    error
}

func (m *mockNetClient) GetStatus(addr string) (*quakenet.StatusResponse, error) {
	return m.status, m.err
}

func TestServerNew(t *testing.T) {
	s := NewServer(
		WithDir("/tmp/quake"),
		WithConfigFile("config.yaml"),
		WithAddr(":27960"),
	)
	if s.Dir != "/tmp/quake" {
		t.Errorf("expected dir /tmp/quake, got %s", s.Dir)
	}
	if s.ConfigFile != "config.yaml" {
		t.Errorf("expected config file config.yaml, got %s", s.ConfigFile)
	}
	if s.Addr != ":27960" {
		t.Errorf("expected addr :27960, got %s", s.Addr)
	}
}

func TestServerWriteDefaultConfig(t *testing.T) {
	dir, err := os.MkdirTemp("", "quake-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	s := NewServer(WithDir(dir))
	if err := s.writeDefaultConfig(); err != nil {
		t.Fatalf("failed to write default config: %v", err)
	}

	configPath := filepath.Join(dir, "baseq3/server.cfg")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config file was not created at %s", configPath)
	}
}

func TestServerMonitorMetrics(t *testing.T) {
	mock := &mockNetClient{
		status: &quakenet.StatusResponse{
			Configuration: map[string]string{"mapname": "q3dm7"},
			Players: []quakenet.Player{
				{Name: "player1", Score: 10, Ping: 50},
			},
		},
	}

	s := NewServer(
		WithAddr(":27960"),
		WithNetClient(mock),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.monitorMetrics(ctx, "localhost", "27960")
	time.Sleep(100 * time.Millisecond)
	cancel()
}

func TestWatch_RegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configFile, []byte("test: value"), 0644); err != nil {
		t.Fatal(err)
	}

	logger := zap.NewNop()
	s := NewServer(
		WithConfigFile(configFile),
		WithDir(tmpDir),
		WithDebounceInterval(100*time.Millisecond),
		WithLogger(logger),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := s.watch(ctx)
	if err != nil {
		t.Fatalf("watch failed: %v", err)
	}

	// Update the file
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(configFile, []byte("test: newvalue"), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for change notification
	select {
	case <-ch:
		t.Log("Change detected successfully")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for change notification")
	}
}

func TestWatch_Symlink(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "..data")
	if err := os.Mkdir(dataDir, 0755); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(dataDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("test: value"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create symlink pointing to data directory (simulating ConfigMap)
	symlinkPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.Symlink(configFile, symlinkPath); err != nil {
		t.Fatal(err)
	}

	logger := zap.NewNop()
	s := NewServer(
		WithConfigFile(symlinkPath),
		WithDir(tmpDir),
		WithDebounceInterval(100*time.Millisecond),
		WithLogger(logger),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := s.watch(ctx)
	if err != nil {
		t.Fatalf("watch failed: %v", err)
	}

	// Simulate ConfigMap update by creating new data and updating symlink
	time.Sleep(50 * time.Millisecond)

	newDataDir := filepath.Join(tmpDir, "..data_new")
	if err := os.Mkdir(newDataDir, 0755); err != nil {
		t.Fatal(err)
	}

	newConfigFile := filepath.Join(newDataDir, "config.yaml")
	if err := os.WriteFile(newConfigFile, []byte("test: newvalue"), 0644); err != nil {
		t.Fatal(err)
	}

	// Remove old symlink and create new one
	if err := os.Remove(symlinkPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(newConfigFile, symlinkPath); err != nil {
		t.Fatal(err)
	}

	// Wait for change notification
	select {
	case <-ch:
		t.Log("ConfigMap change detected successfully")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ConfigMap change notification")
	}
}

func TestGetWatchPath(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configFile, []byte("test: value"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewServer(
		WithConfigFile(configFile),
		WithDir(tmpDir),
	)

	watchPath, err := s.getWatchPath()
	if err != nil {
		t.Fatalf("getWatchPath failed: %v", err)
	}

	if watchPath != configFile {
		t.Errorf("expected watch path %s, got %s", configFile, watchPath)
	}
}
