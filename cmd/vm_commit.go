package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

func vmCommitCommand() *cobra.Command {
	var commitCmd = &cobra.Command{
		Use:   "commit name",
		Short: "Commit a virtual machine",
		Long: `Commit a virtual machine to an image. The image name will be
the virtual machine name.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}

			err = v.VMCommit(args[0])
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	return commitCmd
}
