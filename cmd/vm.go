package cmd

import (
	"github.com/spf13/cobra"
)

func vmCommand() *cobra.Command {
	vmCmd := &cobra.Command{
		Use:   "vm",
		Short: "Virtual machine related subcommands",
		Long:  `Virtual machine related subcommands.`,
	}

	vmCmd.AddCommand(vmCommitCommand())
	vmCmd.AddCommand(vmRmCommand())
	vmCmd.AddCommand(vmRunCommand())
	return vmCmd
}
