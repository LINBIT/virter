package vm

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	connect "github.com/LINBIT/virter/cmd"
	"github.com/LINBIT/virter/pkg/isogenerator"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start a virtual machine",
	Long:  `Start a fresh virtual machine from an image.`,
	Run:   vmRun,
}

func init() {
	vmCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&imageName, "image", "i", "", "image to use")
	runCmd.MarkFlagRequired("image")
	runCmd.Flags().StringVarP(&vmName, "name", "n", "", "name of new VM")
	runCmd.MarkFlagRequired("name")
}

func vmRun(cmd *cobra.Command, args []string) {
	v, err := connect.VirterConnect()
	if err != nil {
		log.Fatal(err)
	}

	sshPublicKey := viper.GetString("auth.ssh_public_key")

	err = v.VMRun(
		isogenerator.ExternalISOGenerator{},
		imageName,
		vmName,
		sshPublicKey)
	if err != nil {
		log.Fatal(err)
	}
}
