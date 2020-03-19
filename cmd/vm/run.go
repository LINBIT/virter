package vm

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	connect "github.com/LINBIT/virter/cmd"
	"github.com/LINBIT/virter/internal/virter"
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
	runCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID for VM which determines the IP address")
	runCmd.MarkFlagRequired("id")
}

func vmRun(cmd *cobra.Command, args []string) {
	v, err := connect.VirterConnect()
	if err != nil {
		log.Fatal(err)
	}

	if vmName == "" {
		vmName = fmt.Sprintf("%s-%d", imageName, vmID)
	}

	sshPublicKey := viper.GetString("auth.ssh_public_key")

	c := virter.VMConfig{
		ImageName:    imageName,
		VMName:       vmName,
		VMID:         vmID,
		SSHPublicKey: sshPublicKey,
	}
	err = v.VMRun(isogenerator.ExternalISOGenerator{}, c)
	if err != nil {
		log.Fatal(err)
	}
}
