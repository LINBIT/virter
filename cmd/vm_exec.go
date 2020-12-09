package cmd

import (
	"context"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/LINBIT/containerapi"
	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/netcopy"

	"github.com/spf13/cobra"
)

func vmExecCommand() *cobra.Command {
	var provisionFile string
	var provisionOverrides []string

	execCmd := &cobra.Command{
		Use:   "exec vm_name [vm_name...]",
		Short: "Run a Docker container against a VM",
		Long:  `Run a Docker container on the host with a connection to a VM.`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := onInterruptWrap(context.Background())
			defer cancel()

			provOpt := virter.ProvisionOption{
				FilePath:  provisionFile,
				Overrides: provisionOverrides,
			}
			if err := execProvision(ctx, provOpt, args); err != nil {
				log.Fatal(err)
			}
		},
	}

	execCmd.Flags().StringVarP(&provisionFile, "provision", "p", "", "name of toml file containing provisioning steps")
	execCmd.Flags().StringSliceVarP(&provisionOverrides, "set", "s", []string{}, "set/override provisioning steps")

	return execCmd
}

func execProvision(ctx context.Context, provOpt virter.ProvisionOption, vmNames []string) error {
	pc, err := virter.NewProvisionConfig(provOpt)
	if err != nil {
		return err
	}
	v, err := InitVirter()
	if err != nil {
		log.Fatal(err)
	}
	defer v.ForceDisconnect()

	for _, s := range pc.Steps {
		if s.Docker != nil {
			if err := execDocker(ctx, v, s.Docker, vmNames); err != nil {
				return err
			}
		} else if s.Shell != nil {
			if err := v.VMExecShell(ctx, vmNames, s.Shell); err != nil {
				return err
			}
		} else if s.Rsync != nil {
			copier := netcopy.NewRsyncNetworkCopier()
			if err := v.VMExecRsync(ctx, copier, vmNames, s.Rsync); err != nil {
				return err
			}
		}
	}

	return nil
}

func execDocker(ctx context.Context, v *virter.Virter, s *virter.ProvisionDockerStep, vmNames []string) error {
	containerProvider, err := containerapi.NewProvider(ctx, containerProvider())
	if err != nil {
		return err
	}
	defer containerProvider.Close()

	containerCfg := containerapi.NewContainerConfig("virter-"+strings.Join(vmNames, "-"), s.Image, s.Env, containerapi.WithCommand(s.Command...))

	return v.VMExecDocker(ctx, containerProvider, vmNames, containerCfg, s.Copy)
}
