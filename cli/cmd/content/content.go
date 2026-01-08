package content

import (
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

var opts struct {
	Addr           string
	AssetsDir      string
	SeedContentURL string
}

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "content",
		Short:         "q3 content server",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if !filepath.IsAbs(opts.AssetsDir) {
				opts.AssetsDir, err = filepath.Abs(opts.AssetsDir)
				if err != nil {
					return err
				}
			}

			if err := os.MkdirAll(opts.AssetsDir, 0755); err != nil {
				return err
			}

			if opts.SeedContentURL != "" {
				u, err := url.Parse(opts.SeedContentURL)
				if err != nil {
					return err
				}
				if err := content.CopyAssets(u, opts.AssetsDir); err != nil {
					return err
				}
			}

			e, err := content.NewRouter(&content.Config{AssetsDir: opts.AssetsDir})
			if err != nil {
				return err
			}
			s := &http.Server{
				Addr:           opts.Addr,
				Handler:        e,
				ReadTimeout:    600 * time.Second,
				WriteTimeout:   600 * time.Second,
				MaxHeaderBytes: 1 << 20,
			}
			logger.DefaultLogger.Info("starting server", zap.String("addr", opts.Addr))
			return s.ListenAndServe()
		},
	}
	cmd.Flags().StringVarP(&opts.Addr, "addr", "a", ":9090", "address <host>:<port>")
	cmd.Flags().StringVarP(&opts.AssetsDir, "assets-dir", "d", "assets", "assets directory")
	cmd.Flags().StringVar(&opts.SeedContentURL, "seed-content-url", "", "seed content from another content server")
	return cmd
}
