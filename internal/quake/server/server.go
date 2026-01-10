package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	quakenet "github.com/grahamplata/quake-kube/internal/quake/net"
	"github.com/grahamplata/quake-kube/pkg/exec"
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

// WithDebounceInterval sets the debounce interval for config file changes.
func WithDebounceInterval(interval time.Duration) Option {
	return func(s *Server) {
		s.DebounceInterval = interval
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
	// Dir is the directory where user-specific game data
	Dir string
	// DebounceInterval is the interval to wait after the last file change before reloading.
	DebounceInterval time.Duration
	// ConfigFile is the config file for the server.
	ConfigFile string
	// Addr is the address for the server.
	Addr string
	// Logger is the logger for the server.
	Logger *zap.Logger
	// netClient is the net client for the server.
	netClient NetClient
}

// NewServer creates a new Server.
func NewServer(options ...Option) *Server {
	s := &Server{
		DebounceInterval: 2 * time.Second,
		netClient:        &defaultNetClient{},
		Logger:           zap.NewNop(),
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
		return fmt.Errorf("failed to split host and port: %v", err)
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
			return fmt.Errorf("failed to write default config: %v", err)
		}
	} else {
		if err := s.reload(); err != nil {
			return fmt.Errorf("failed to reload config: %v", err)
		}
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start: %v", err)
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
		return fmt.Errorf("failed to marshal default config: %v", err)
	}
	configPath := filepath.Join(s.Dir, "baseq3", "server.cfg")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
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
				return fmt.Errorf("failed to restart server: %v", err)
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
		return fmt.Errorf("failed to read config file: %v", err)
	}
	cfg := WithDefault()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config file: %v", err)
	}
	data, err = cfg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal config file: %v", err)
	}
	output := filepath.Join(s.Dir, "baseq3/server.cfg")
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	return os.WriteFile(output, data, 0644)
}

// watch sets up file watching with fsnotify.
// It handles both regular files and Kubernetes ConfigMap symlinks.
func (s *Server) watch(ctx context.Context) (<-chan struct{}, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	// Determine what to watch
	watchPath, err := s.getWatchPath()
	if err != nil {
		defer func() {
			if err := watcher.Close(); err != nil {
				fmt.Printf("failed to close watcher: %v\n", err)
			}
		}()
		return nil, fmt.Errorf("failed to get watch path: %v", err)
	}

	if err := watcher.Add(watchPath); err != nil {
		defer func() {
			if err := watcher.Close(); err != nil {
				fmt.Printf("failed to close watcher: %v\n", err)
			}
		}()
		return nil, fmt.Errorf("failed to watch %s: %w", watchPath, err)
	}

	s.Logger.Info("watching for config changes",
		zap.String("path", watchPath),
		zap.String("config_file", s.ConfigFile))

	ch := make(chan struct{})

	go func() {
		defer func() {
			if err := watcher.Close(); err != nil {
				fmt.Printf("failed to close watcher: %v\n", err)
			}
		}()

		var debounceTimer *time.Timer
		var timerCh <-chan time.Time

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Check if this event is relevant to our config file
				if !s.isRelevantEvent(event, watchPath) {
					continue
				}

				s.Logger.Debug("file event received",
					zap.String("name", event.Name),
					zap.String("op", event.Op.String()))

				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.NewTimer(s.DebounceInterval)
				timerCh = debounceTimer.C

			case <-timerCh:
				// Debounce period elapsed, trigger reload
				select {
				case ch <- struct{}{}:
				default:
					// Channel blocked, skip this reload
				}
				timerCh = nil

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				s.Logger.Warn("watcher error", zap.Error(err))

			case <-ctx.Done():
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				return
			}
		}
	}()

	return ch, nil
}

// getWatchPath determines what path to watch based on whether the config file is a symlink (ConfigMap) or a regular file.
func (s *Server) getWatchPath() (string, error) {
	fi, err := os.Lstat(s.ConfigFile)
	if err != nil {
		return "", fmt.Errorf("failed to stat config file: %w", err)
	}

	// If it's a symlink (typical for ConfigMaps), watch the parent directory
	if fi.Mode()&os.ModeSymlink != 0 {
		dir := filepath.Dir(s.ConfigFile)
		s.Logger.Info("config file is a symlink, watching parent directory",
			zap.String("dir", dir))
		return dir, nil
	}

	return s.ConfigFile, nil
}

// isRelevantEvent checks if a filesystem event is relevant to our config file.
func (s *Server) isRelevantEvent(event fsnotify.Event, watchPath string) bool {
	// If we're watching a directory (ConfigMap case), check if the event
	// is related to our config file or the ..data symlink that ConfigMaps use
	if watchPath != s.ConfigFile {
		basename := filepath.Base(s.ConfigFile)
		eventBasename := filepath.Base(event.Name)

		// ConfigMaps update by creating a new ..data_tmp, then atomically
		// renaming it to ..data. Watch for ..data changes.
		if eventBasename == "..data" {
			return true
		}

		// Also watch for direct file updates
		if eventBasename == basename {
			return true
		}

		// Check if the event is for a new ..data_tmp being created
		if event.Op&fsnotify.Create != 0 && eventBasename == "..data_tmp" {
			return false // Wait for the rename
		}

		return false
	}

	// If watching the file directly, all write/create/remove events are relevant
	return event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0
}
