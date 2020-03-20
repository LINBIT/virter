package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/isogenerator"
)

func vmRunCommand() *cobra.Command {
	var imageName, vmName string
	var vmID uint

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Start a virtual machine",
		Long:  `Start a fresh virtual machine from an image.`,
		Run: func(cmd *cobra.Command, args []string) {
			v, err := VirterConnect()
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
		},
	}

	runCmd.Flags().StringVarP(&imageName, "image", "i", "", "image to use")
	runCmd.MarkFlagRequired("image")
	runCmd.Flags().StringVarP(&vmName, "name", "n", "", "name of new VM")
	runCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID for VM which determines the IP address")
	runCmd.MarkFlagRequired("id")

	return runCmd
}
