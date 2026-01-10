package content

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/grahamplata/quake-kube/internal/quake/content"
	"github.com/grahamplata/quake-kube/pkg/logger"
	"go.uber.org/zap"
)

func NewCommand() *cobra.Command {
	var addr, assetsDir, seedContentURL string
	cmd := &cobra.Command{
		Use:           "content",
		Short:         "q3 content server",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if !filepath.IsAbs(assetsDir) {
				assetsDir, err = filepath.Abs(assetsDir)
				if err != nil {
					return fmt.Errorf("failed to get absolute path for assets directory: %w", err)
				}
			}

			if err := os.MkdirAll(assetsDir, 0755); err != nil {
				return fmt.Errorf("failed to create assets directory: %w", err)
			}

			if seedContentURL != "" {
				u, err := url.Parse(seedContentURL)
				if err != nil {
					return fmt.Errorf("failed to parse seed content URL: %w", err)
				}
				if err := content.CopyAssets(u, assetsDir); err != nil {
					return fmt.Errorf("failed to copy assets: %w", err)
				}
			}

			e, err := content.NewRouter(&content.Config{AssetsDir: assetsDir})
			if err != nil {
				return fmt.Errorf("failed to create router: %w", err)
			}
			s := &http.Server{
				Addr:           addr,
				Handler:        e,
				ReadTimeout:    600 * time.Second,
				WriteTimeout:   600 * time.Second,
				MaxHeaderBytes: 1 << 20,
			}
			l, err := logger.NewLogger(logger.Config{
				LogLevel:    "info",
				ServiceName: "q3-content",
			})
			if err != nil {
				return fmt.Errorf("failed to create logger: %w", err)
			}

			l.Info("starting server", zap.String("addr", addr))
			return s.ListenAndServe()
		},
	}
	cmd.Flags().StringVarP(&addr, "addr", "a", ":9090", "address <host>:<port>")
	cmd.Flags().StringVarP(&assetsDir, "assets-dir", "d", "assets", "assets directory")
	cmd.Flags().StringVar(&seedContentURL, "seed-content-url", "", "seed content from another content server")

	return cmd
}
