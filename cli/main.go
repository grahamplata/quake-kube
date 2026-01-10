package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/grahamplata/quake-kube/cli/cmd/content"
	"github.com/grahamplata/quake-kube/cli/cmd/proxy"
	"github.com/grahamplata/quake-kube/cli/cmd/server"
	"github.com/grahamplata/quake-kube/pkg/logger"
)

var version = "dev"

var global struct {
	Verbosity int
}

func main() {
	cmd := &cobra.Command{
		Use:     "q3",
		Version: version,
	}
	cmd.AddCommand(
		content.NewCommand(),
		proxy.NewCommand(),
		server.NewCommand(),
	)

	cmd.PersistentFlags().CountVarP(&global.Verbosity, "verbose", "v", "log output verbosity")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		lvl := "info"
		if global.Verbosity > 0 {
			lvl = "debug"
		}
		log, err := logger.NewLogger(logger.Config{LogLevel: lvl, ServiceName: "q3"})
		if err != nil {
			return fmt.Errorf("failed to create logger: %w", err)
		}
		logger.DefaultLogger = log
		return nil
	}

	if err := cmd.Execute(); err != nil {
		logger.DefaultLogger.Fatal(err.Error())
	}
}
