package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

func vmCommitCommand() *cobra.Command {
	var vmName string

	var commitCmd = &cobra.Command{
		Use:   "commit",
		Short: "Commit a virtual machine",
		Long: `Commit a virtual machine to an image. The image name will be
the virtual machine name.`,
		Run: func(cmd *cobra.Command, args []string) {

			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}

			err = v.VMCommit(vmName)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	commitCmd.Flags().StringVarP(&vmName, "name", "n", "", "name of the VM to commit")
	commitCmd.MarkFlagRequired("name")

	return commitCmd
}
