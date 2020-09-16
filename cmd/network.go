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
	return networkCmd
}
