package vm

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/LINBIT/virter/internal/connect"
	"github.com/LINBIT/virter/pkg/isogenerator"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start a virtual machine",
	Long:  `Start a fresh virtual machine from an image.`,
	Run:   vmRun,
}

var imageName string
var vmName string

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

	err = v.VMRun(
		isogenerator.ExternalISOGenerator{},
		imageName,
		vmName)
	if err != nil {
		log.Fatal(err)
	}
}
