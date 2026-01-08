package proxy

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

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
			if clientAddr == "" {
				hostIPv4, err := net.DetectHostIPv4()
				if err != nil {
					return err
				}
				clientAddr = fmt.Sprintf("%s:8080", hostIPv4)
			}

			l, err := logger.NewLogger(logger.Config{
				LogLevel:    "info",
				ServiceName: "q3-proxy",
			})
			if err != nil {
				return err
			}

			p, err := client.NewProxy(serverAddr, l)
			if err != nil {
				return err
			}

			s := http.Server{Addr: clientAddr, Handler: p}

			return s.ListenAndServe()
		},
	}

	cmd.Flags().StringVarP(&clientAddr, "client-addr", "c", "", "client address <host>:<port>")
	cmd.Flags().StringVarP(&serverAddr, "server-addr", "s", "", "dedicated server <host>:<port>")

	return cmd
}
