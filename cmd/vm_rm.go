package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func vmRmCommand() *cobra.Command {
	rmCmd := &cobra.Command{
		Use:   "rm name",
		Short: "Remove a virtual machine given a VM name",
		Long:  `Remove a virtual machine including all data.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}

			err = v.VMRm(args[0])
			if err != nil {
				log.Fatal(err)
			}

		},
	}

	return rmCmd
}
