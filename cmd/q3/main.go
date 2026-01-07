package main

import (
	"github.com/spf13/cobra"

	q3cmd "github.com/criticalstack/quake-kube/cmd/q3/app/cmd"
	q3content "github.com/criticalstack/quake-kube/cmd/q3/app/content"
	q3proxy "github.com/criticalstack/quake-kube/cmd/q3/app/proxy"
	q3server "github.com/criticalstack/quake-kube/cmd/q3/app/server"
	"github.com/criticalstack/quake-kube/pkg/logger"
)

var global struct {
	Verbosity int
}

func main() {
	cmd := &cobra.Command{
		Use:   "q3",
		Short: "",
	}
	cmd.AddCommand(
		q3cmd.NewCommand(),
		q3content.NewCommand(),
		q3proxy.NewCommand(),
		q3server.NewCommand(),
	)

	cmd.PersistentFlags().CountVarP(&global.Verbosity, "verbose", "v", "log output verbosity")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		lvl := "info"
		if global.Verbosity > 0 {
			lvl = "debug"
		}
		l, err := logger.NewLogger(logger.Config{
			LogLevel:    lvl,
			ServiceName: "q3",
		})
		if err != nil {
			return err
		}
		logger.DefaultLogger = l
		return nil
	}

	if err := cmd.Execute(); err != nil {
		logger.DefaultLogger.Fatal(err.Error())
	}
}
