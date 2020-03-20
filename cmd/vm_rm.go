package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

func vmRmCommand() *cobra.Command {
	var vmName string

	rmCmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove a virtual machine",
		Long:  `Remove a virtual machine including all data.`,
		Run: func(cmd *cobra.Command, args []string) {
			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}

			err = v.VMRm(vmName)
			if err != nil {
				log.Fatal(err)
			}

		},
	}

	rmCmd.Flags().StringVarP(&vmName, "name", "n", "", "name of the VM to remove")
	rmCmd.MarkFlagRequired("name")

	return rmCmd
}
