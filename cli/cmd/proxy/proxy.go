package proxy

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/grahamplata/quake-kube/internal/quake/client"
	"github.com/grahamplata/quake-kube/pkg/logger"
	"github.com/grahamplata/quake-kube/pkg/net"
)

func NewCommand() *cobra.Command {
	var clientAddr, serverAddr string

	cmd := &cobra.Command{
		Use:          "proxy",
		Short:        "q3 websocket/udp proxy",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Set up signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			if clientAddr == "" {
				hostIPv4, err := net.DetectHostIPv4()
				if err != nil {
					return fmt.Errorf("failed to detect host IPv4: %w", err)
				}
				clientAddr = fmt.Sprintf("%s:8080", hostIPv4)
			}

			l, err := logger.NewLogger(logger.Config{
				LogLevel:    "info",
				ServiceName: "q3-proxy",
			})
			if err != nil {
				return fmt.Errorf("failed to create logger: %w", err)
			}

			// Handle shutdown signal
			go func() {
				sig := <-sigChan
				l.Info("received signal, initiating graceful shutdown", zap.String("signal", sig.String()))
				cancel()
			}()

			p, err := client.NewProxy(serverAddr, l)
			if err != nil {
				return fmt.Errorf("failed to create proxy: %w", err)
			}

			s := &http.Server{Addr: clientAddr, Handler: p}

			// Start server in goroutine
			errChan := make(chan error, 1)
			go func() {
				l.Info("starting proxy", zap.String("addr", clientAddr))
				if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					errChan <- err
				}
			}()

			// Wait for shutdown or error
			select {
			case <-ctx.Done():
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer shutdownCancel()
				return s.Shutdown(shutdownCtx)
			case err := <-errChan:
				return fmt.Errorf("proxy failed: %w", err)
			}
		},
	}

	cmd.Flags().StringVarP(&clientAddr, "client-addr", "c", "", "client address <host>:<port>")
	cmd.Flags().StringVarP(&serverAddr, "server-addr", "s", "", "dedicated server <host>:<port>")

	return cmd
}
