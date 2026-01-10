package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/grahamplata/quake-kube/internal/quake/client"
	"github.com/grahamplata/quake-kube/internal/quake/content"
	"github.com/grahamplata/quake-kube/internal/quake/server"
	"github.com/grahamplata/quake-kube/pkg/logger"
	"github.com/grahamplata/quake-kube/public"
)

var (
	// Quake 3 Arena demo
	// https://lvlworld.com/misc-files/Q3A-Demo-Linux.zip
	demoURL = "https://lvlworld.com/misc-files/Q3A-Demo-Linux.zip"

	// PAK file only upgrade - for unofficial game engines
	// https://lvlworld.com/misc-files/Point-Release-1.32-PAK.zip
	pakURL = "https://lvlworld.com/misc-files/Point-Release-1.32-PAK.zip"
)

func NewCommand() *cobra.Command {
	var clientAddr, serverAddr, contentServer, assetsDir, configFile string
	var acceptEula, noAssetsDownload bool
	var debounceInterval time.Duration

	cmd := &cobra.Command{
		Use:          "server",
		Short:        "q3 server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Set up signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			// Must accept Eula to proceed
			if !acceptEula {
				fmt.Print(server.Q3DemoEULA)
				return errors.New("you must agree to the EULA to continue")
			}

			logger, err := logger.NewLogger(logger.Config{
				LogLevel:    "info",
				ServiceName: "q3-server",
			})
			if err != nil {
				return fmt.Errorf("failed to create logger: %w", err)
			}

			// Handle shutdown signal
			go func() {
				sig := <-sigChan
				logger.Info("received signal, initiating graceful shutdown", zap.String("signal", sig.String()))
				cancel()
			}()

			// Download free ioquake3 point release files (pak1-pak8)
			// These are freely available and don't require purchasing Quake 3
			if !noAssetsDownload {
				if err := content.DownloadAssetsFromURLs(demoURL, pakURL, assetsDir); err != nil {
					logger.Warn("failed to download assets", zap.Error(err))
				}
			}

			go func() {
				s := server.NewServer(
					server.WithDir(assetsDir),
					server.WithDebounceInterval(debounceInterval),
					server.WithConfigFile(configFile),
					server.WithAddr(serverAddr),
					server.WithLogger(logger),
				)
				if err := s.Start(ctx); err != nil {
					logger.Fatal("server failed", zap.Error(err))
				}
			}()

			e, err := client.NewRouter(&client.Config{
				ContentServerURL: contentServer,
				ServerAddr:       serverAddr,
				Files:            public.Files,
			})
			if err != nil {
				logger.Fatal("failed to create router", zap.Error(err))
			}

			s := &client.Server{Addr: clientAddr, Handler: e, ServerAddr: serverAddr, Logger: logger}

			logger.Info("starting server", zap.String("addr", clientAddr))
			return s.ListenAndServe(ctx)
		},
	}

	cmd.Flags().StringVarP(&configFile, "config", "c", "", "server configuration file")
	cmd.Flags().StringVar(&contentServer, "content-server", "http://content.quakejs.com", "content server url")
	cmd.Flags().BoolVar(&acceptEula, "agree-eula", false, "agree to the Quake 3 demo EULA")
	cmd.Flags().StringVar(&assetsDir, "assets-dir", "assets", "location for game files")
	cmd.Flags().StringVar(&clientAddr, "client-addr", "0.0.0.0:8080", "client address <host>:<port>")
	cmd.Flags().StringVar(&serverAddr, "server-addr", "0.0.0.0:27960", "dedicated server <host>:<port>")
	cmd.Flags().DurationVar(&debounceInterval, "debounce-interval", 2*time.Second, "debounce interval for config file changes")
	cmd.Flags().BoolVar(&noAssetsDownload, "no-assets-download", false, "skip downloading assets from content server")

	return cmd
}
