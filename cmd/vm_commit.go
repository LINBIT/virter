package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v7"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/actualtime"
)

func vmCommitCommand() *cobra.Command {
	var resetMachineID bool
	var shutdown bool

	var commitCmd = &cobra.Command{
		Use:   "commit vm-name [image-name]",
		Short: "Commit a virtual machine",
		Long: `Commit a virtual machine to an image. The image name will be
the virtual machine name if no other value is given.`,
		Args: cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			shutdownTimeout := viper.GetDuration("time.shutdown_timeout")

			vmName := args[0]
			imageName := vmName
			if len(args) == 2 {
				imageName = args[1]
			}

			ctx := cmd.Context()

			p := mpb.NewWithContext(ctx, DefaultContainerOpt())

			commitConfig := virter.CommitConfig{
				ImageName:       imageName,
				Shutdown:        shutdown,
				ShutdownTimeout: shutdownTimeout,
				ResetMachineID:  resetMachineID,
			}

			err = v.VMCommit(ctx, actualtime.ActualTime{}, vmName, commitConfig, viper.GetBool("libvirt.static_dhcp"), virter.WithProgress(DefaultProgressFormat(p)))
			if err != nil {
				log.Fatal(err)
			}
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return suggestVmNames(cmd, args, toComplete)
			}

			if len(args) == 1 {
				return suggestImageNames(cmd, args, toComplete)
			}

			return suggestNone(cmd, args, toComplete)
		},
	}

	commitCmd.Flags().BoolVar(&resetMachineID, "reset-machine-id", true, "Whether or not to clear the /etc/machine-id file after provisioning")
	commitCmd.Flags().BoolVarP(&shutdown, "shutdown", "s", true, "Whether to shut the VM down and wait until it stops")

	return commitCmd
}
