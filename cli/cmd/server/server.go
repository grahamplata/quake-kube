package server

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/grahamplata/quake-kube/internal/quake/client"
	"github.com/grahamplata/quake-kube/internal/quake/content"
	"github.com/grahamplata/quake-kube/internal/quake/server"
	"github.com/grahamplata/quake-kube/pkg/logger"
	"github.com/grahamplata/quake-kube/pkg/net/http"
	"github.com/grahamplata/quake-kube/public"
)

var opts struct {
	ClientAddr       string
	ServerAddr       string
	ContentServer    string
	AcceptEula       bool
	AssetsDir        string
	ConfigFile       string
	WatchInterval    time.Duration
	NoAssetsDownload bool
}

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "server",
		Short:        "q3 server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			csurl, err := url.Parse(opts.ContentServer)
			if err != nil {
				return err
			}
			if !opts.AcceptEula {
				fmt.Print(server.Q3DemoEULA)
				return errors.New("You must agree to the EULA to continue")
			}

			// Download free ioquake3 point release files (pak1-pak8)
			// These are freely available and don't require purchasing Quake 3
			if !opts.NoAssetsDownload {
				// Optionally try to download additional assets from content server
				// This is now optional since the free point release files should be sufficient
				if opts.ContentServer != "" && opts.ContentServer != "http://content.quakejs.com" {
					if err := http.GetUntil(opts.ContentServer, ctx.Done()); err == nil {
						// Only attempt if server is reachable
						if err := content.CopyAssets(csurl, opts.AssetsDir); err != nil {
							// Log but don't fail - we have the free files already
							logger.DefaultLogger.Warn("failed to copy assets from content server", zap.Error(err))
						}
					}
				}
			}

			logger, err := logger.NewLogger(logger.Config{
				LogLevel:    "info",
				ServiceName: "q3-server",
			})
			if err != nil {
				return err
			}

			go func() {
				s := server.NewServer(
					server.WithDir(opts.AssetsDir),
					server.WithWatchInterval(opts.WatchInterval),
					server.WithConfigFile(opts.ConfigFile),
					server.WithAddr(opts.ServerAddr),
					server.WithLogger(logger),
				)
				if err := s.Start(ctx); err != nil {
					logger.Fatal("server failed", zap.Error(err))
				}
			}()

			e, err := client.NewRouter(&client.Config{
				ContentServerURL: opts.ContentServer,
				ServerAddr:       opts.ServerAddr,
				Files:            public.Files,
			})
			if err != nil {
				logger.Fatal("failed to create router", zap.Error(err))
			}

			s := &client.Server{
				Addr:       opts.ClientAddr,
				Handler:    e,
				ServerAddr: opts.ServerAddr,
			}

			logger.Info("starting server", zap.String("addr", opts.ClientAddr))
			return s.ListenAndServe()
		},
	}

	cmd.Flags().StringVarP(&opts.ConfigFile, "config", "c", "", "server configuration file")
	cmd.Flags().StringVar(&opts.ContentServer, "content-server", "http://content.quakejs.com", "content server url")
	cmd.Flags().BoolVar(&opts.AcceptEula, "agree-eula", false, "agree to the Quake 3 demo EULA")
	cmd.Flags().StringVar(&opts.AssetsDir, "assets-dir", "assets", "location for game files")
	cmd.Flags().StringVar(&opts.ClientAddr, "client-addr", "0.0.0.0:8080", "client address <host>:<port>")
	cmd.Flags().StringVar(&opts.ServerAddr, "server-addr", "0.0.0.0:27960", "dedicated server <host>:<port>")
	cmd.Flags().DurationVar(&opts.WatchInterval, "watch-interval", 15*time.Second, "dedicated server <host>:<port>")
	cmd.Flags().BoolVar(&opts.NoAssetsDownload, "no-assets-download", false, "skip downloading assets from content server")

	return cmd
}
