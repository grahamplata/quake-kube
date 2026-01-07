package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	quakenet "github.com/criticalstack/quake-kube/internal/quake/net"
	"github.com/criticalstack/quake-kube/pkg/exec"
)

// NetClient is an interface for interacting with the Quake server.
type NetClient interface {
	GetStatus(addr string) (*quakenet.StatusResponse, error)
}

// defaultNetClient is a default implementation of NetClient.
type defaultNetClient struct{}

// GetStatus implements NetClient.
func (d *defaultNetClient) GetStatus(addr string) (*quakenet.StatusResponse, error) {
	return quakenet.GetStatus(addr)
}

// Option is a functional option for the Server.
type Option func(*Server)

// WithDir sets the directory for the server.
func WithDir(dir string) Option {
	return func(s *Server) {
		s.Dir = dir
	}
}

// WithConfigFile sets the config file for the server.
func WithConfigFile(configFile string) Option {
	return func(s *Server) {
		s.ConfigFile = configFile
	}
}

// WithAddr sets the address for the server.
func WithAddr(addr string) Option {
	return func(s *Server) {
		s.Addr = addr
	}
}

// WithWatchInterval sets the watch interval for the server.
func WithWatchInterval(interval time.Duration) Option {
	return func(s *Server) {
		s.WatchInterval = interval
	}
}

// WithNetClient sets the net client for the server.
func WithNetClient(netClient NetClient) Option {
	return func(s *Server) {
		s.netClient = netClient
	}
}

// WithLogger sets the logger for the server.
func WithLogger(logger *zap.Logger) Option {
	return func(s *Server) {
		s.Logger = logger
	}
}

func WithDefault() *Config {
	return &Config{
		Overrides: []string{},
		FragLimit: 25,
		TimeLimit: metav1.Duration{Duration: 15 * time.Minute},
		Maps: []Map{
			{Name: "q3dm7", Type: FreeForAll},
			{Name: "q3dm17", Type: FreeForAll},
		},
		GameConfig: GameConfig{
			Log:           "",
			MOTD:          "Welcome to QuakeKube!",
			QuadFactor:    3,
			GameType:      FreeForAll,
			WeaponRespawn: 3,
			Inactivity:    metav1.Duration{Duration: 10 * time.Minute},
			ForceRespawn:  false,
			JoinPassword:  "",
		},
		BotConfig: BotConfig{
			Skill:  2,
			NoChat: true,
		},
		ServerConfig: ServerConfig{
			MaxClients:   12,
			Hostname:     "quakekube",
			RconPassword: "changeme",
		},
	}
}

// Server represents a Quake server.
type Server struct {
	Dir           string
	WatchInterval time.Duration
	ConfigFile    string
	Addr          string
	Logger        *zap.Logger

	netClient NetClient
}

// NewServer creates a new Server.
func NewServer(options ...Option) *Server {
	s := &Server{
		WatchInterval: 15 * time.Second,
		netClient:     &defaultNetClient{},
		Logger:        zap.NewNop(),
	}

	for _, option := range options {
		option(s)
	}
	return s
}

// Start starts the server.
func (s *Server) Start(ctx context.Context) error {
	if s.Addr == "" {
		s.Addr = "0.0.0.0:27960"
	}
	host, port, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return err
	}

	args := []string{
		"+set", "dedicated", "1",
		"+set", "net_ip", host,
		"+set", "net_port", port,
		"+set", "com_homepath", s.Dir,
		"+set", "com_basegame", "baseq3",
		"+set", "com_gamename", "Quake3Arena",
		"+exec", "server.cfg",
	}

	cmd := exec.CommandContext(ctx, "ioq3ded", args...)
	cmd.Dir = s.Dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if s.ConfigFile == "" {
		if err := s.writeDefaultConfig(); err != nil {
			return err
		}
	} else {
		if err := s.reload(); err != nil {
			return err
		}
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait group or multiple goroutines for monitoring
	go s.monitorProcess(cmd)
	go s.monitorMetrics(ctx, host, port)

	if s.ConfigFile == "" {
		return cmd.Wait()
	}

	return s.watchAndReload(ctx, cmd)
}

func (s *Server) writeDefaultConfig() error {
	cfg := WithDefault()
	data, err := cfg.Marshal()
	if err != nil {
		return err
	}
	configPath := filepath.Join(s.Dir, "baseq3", "server.cfg")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func (s *Server) monitorProcess(cmd *exec.Cmd) {
	if err := cmd.Wait(); err != nil {
		s.Logger.Error("server process exited with error", zap.Error(err))
	}
}

func (s *Server) watchAndReload(ctx context.Context, cmd *exec.Cmd) error {
	ch, err := s.watch(ctx)
	if err != nil {
		return fmt.Errorf("failed to watch config file: %v", err)
	}

	for {
		select {
		case <-ch:
			s.Logger.Info("config change detected, reloading...")
			if err := s.reload(); err != nil {
				s.Logger.Error("reload failed", zap.Error(err))
				continue
			}
			configReloads.Inc()
			if err := cmd.Restart(ctx); err != nil {
				return err
			}
			go s.monitorProcess(cmd)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Server) reload() error {
	data, err := os.ReadFile(s.ConfigFile)
	if err != nil {
		return err
	}
	cfg := WithDefault()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	data, err = cfg.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, "baseq3/server.cfg"), data, 0644)
}

func (s *Server) watch(ctx context.Context) (<-chan struct{}, error) {
	if s.WatchInterval == 0 {
		s.WatchInterval = 15 * time.Second
	}
	fi, err := os.Stat(s.ConfigFile)
	if err != nil {
		return nil, err
	}
	curModTime := fi.ModTime()

	ch := make(chan struct{})

	go func() {
		ticker := time.NewTicker(s.WatchInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if fi, err := os.Stat(s.ConfigFile); err == nil {
					if fi.ModTime().After(curModTime) {
						curModTime = fi.ModTime()
						select {
						case ch <- struct{}{}:
						default:
						}
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}
