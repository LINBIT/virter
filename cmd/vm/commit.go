package vm

import (
	"log"

	"github.com/spf13/cobra"

	connect "github.com/LINBIT/virter/cmd"
)

// commitCmd represents the commit command
var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Commit a virtual machine",
	Long: `Commit a virtual machine to an image. The image name will be
the virtual machine name.`,
	Run: vmCommit,
}

func init() {
	vmCmd.AddCommand(commitCmd)

	commitCmd.Flags().StringVarP(&vmName, "name", "n", "", "name of the VM to commit")
	commitCmd.MarkFlagRequired("name")
}

func vmCommit(cmd *cobra.Command, args []string) {
	v, err := connect.VirterConnect()
	if err != nil {
		log.Fatal(err)
	}

	err = v.VMCommit(vmName)
	if err != nil {
		log.Fatal(err)
	}
}
