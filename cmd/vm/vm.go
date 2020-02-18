package vm

import (
	"github.com/spf13/cobra"

	"github.com/LINBIT/virter/cmd"
)

// vmCmd represents the vm command
var vmCmd = &cobra.Command{
	Use:   "vm",
	Short: "Virtual machine related subcommands",
	Long:  `Virtual machine related subcommands.`,
}

func init() {
	cmd.RootCmd.AddCommand(vmCmd)
}

var imageName string
var vmName string
