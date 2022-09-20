package cmd

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func vmExistsCommand() *cobra.Command {
	existsCmd := &cobra.Command{
		Use:   "exists vm_name",
		Short: "Check whether a VM exists",
		Long:  `Check whether a VM exists and was created by Virter.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			if err := v.VMExists(args[0]); err != nil {
				os.Exit(1)
			}
		},
		ValidArgsFunction: suggestVmNames,
	}
	return existsCmd
}
