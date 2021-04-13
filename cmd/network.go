package cmd

import (
	"github.com/spf13/cobra"
)

func networkCommand() *cobra.Command {
	networkCmd := &cobra.Command{
		Use:   "network",
		Short: "Network related subcommands",
		Long:  `Network related subcommands.`,
	}

	networkCmd.AddCommand(networkHostCommand())
	networkCmd.AddCommand(networkLsCommand())
	networkCmd.AddCommand(networkAddCommand())
	networkCmd.AddCommand(networkRmCommand())
	networkCmd.AddCommand(networkListAttachedCommand())
	return networkCmd
}
