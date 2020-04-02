package cmd

import (
	"fmt"
	"log"

	"github.com/rck/unit"
	"github.com/spf13/cobra"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/isogenerator"
)

func vmRunCommand() *cobra.Command {
	var vmName string
	var vmID uint
	var waitSSH bool

	var mem *unit.Value
	var memKiB uint64

	runCmd := &cobra.Command{
		Use:   "run image",
		Short: "Start a virtual machine with a given image",
		Long:  `Start a fresh virtual machine from an image.`,
		Args:  cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			memKiB = uint64(mem.Value / unit.DefaultUnits["K"])
		},
		Run: func(cmd *cobra.Command, args []string) {
			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}

			imageName := args[0]
			if vmName == "" {
				vmName = fmt.Sprintf("%s-%d", imageName, vmID)
			}

			publicKeys, err := loadPublicKeys()
			if err != nil {
				log.Fatal(err)
			}

			c := virter.VMConfig{
				ImageName:     imageName,
				VMName:        vmName,
				MemoryKiB:     memKiB,
				VMID:          vmID,
				SSHPublicKeys: publicKeys,
			}
			err = v.VMRun(isogenerator.ExternalISOGenerator{}, newSSHPinger(), c, waitSSH)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	runCmd.Flags().StringVarP(&vmName, "name", "n", "", "name of new VM")
	runCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID for VM which determines the IP address")
	runCmd.MarkFlagRequired("id")
	runCmd.Flags().BoolVarP(&waitSSH, "wait-ssh", "w", false, "whether to wait for SSH port (default false)")
	units := unit.DefaultUnits
	units["KiB"] = units["K"]
	units["MiB"] = units["M"]
	units["GiB"] = units["G"]
	units["TiB"] = units["T"]
	units["PiB"] = units["P"]
	units["EiB"] = units["E"]
	u := unit.MustNewUnit(units)
	mem = u.MustNewValue(1*units["G"], unit.None)
	runCmd.Flags().VarP(mem, "memory", "m", "Set amount of memory for the VM")

	return runCmd
}
