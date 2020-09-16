package cmd

import (
	"github.com/spf13/cobra"
)

func networkHostCommand() *cobra.Command {
	networkHostCmd := &cobra.Command{
		Use:   "host",
		Short: "Network host related subcommands",
		Long:  `Network host related subcommands.`,
	}

	networkHostCmd.AddCommand(networkHostAddCommand())
	networkHostCmd.AddCommand(networkHostRmCommand())
	return networkHostCmd
}
