package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/isogenerator"
	"github.com/LINBIT/virter/pkg/tcpping"
)

func vmRunCommand() *cobra.Command {
	var vmName string
	var vmID uint
	var waitSSH bool

	runCmd := &cobra.Command{
		Use:   "run image",
		Short: "Start a virtual machine with a given image",
		Long:  `Start a fresh virtual machine from an image.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}

			imageName := args[0]
			if vmName == "" {
				vmName = fmt.Sprintf("%s-%d", imageName, vmID)
			}

			pinger := tcpping.TCPPinger{
				Count:  viper.GetInt("ping.count"),
				Period: viper.GetDuration("ping.period"),
			}

			sshPublicKey := viper.GetString("auth.ssh_public_key")

			c := virter.VMConfig{
				ImageName:    imageName,
				VMName:       vmName,
				VMID:         vmID,
				SSHPublicKey: sshPublicKey,
			}
			err = v.VMRun(isogenerator.ExternalISOGenerator{}, pinger, c, waitSSH)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	runCmd.MarkFlagRequired("image")
	runCmd.Flags().StringVarP(&vmName, "name", "n", "", "name of new VM")
	runCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID for VM which determines the IP address")
	runCmd.MarkFlagRequired("id")
	runCmd.Flags().BoolVarP(&waitSSH, "wait-ssh", "w", false, "whether to wait for SSH port (default false)")

	return runCmd
}
