package cmd

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/LINBIT/containerapi"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/netcopy"
	"github.com/LINBIT/virter/pkg/pullpolicy"

	"github.com/spf13/cobra"
)

func vmExecCommand() *cobra.Command {
	var provisionFile string
	var provisionOverrides []string

	var containerPullPolicy pullpolicy.PullPolicy

	execCmd := &cobra.Command{
		Use:   "exec vm_name [vm_name...]",
		Short: "Run provisioning steps on VMs",
		Long:  `Run provisioning steps. For instance, shell scripts directly on VMs, or from a container with connections to VMs.`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			provOpt := virter.ProvisionOption{
				FilePath:           provisionFile,
				Overrides:          provisionOverrides,
				DefaultPullPolicy:  getDefaultContainerPullPolicy(),
				OverridePullPolicy: containerPullPolicy,
			}
			if err := execProvision(cmd.Context(), provOpt, args); err != nil {
				log.Fatal(err)
			}
		},
		ValidArgsFunction: suggestVmNames,
	}

	execCmd.Flags().StringVarP(&provisionFile, "provision", "p", "", "name of toml file containing provisioning steps")
	execCmd.Flags().StringArrayVarP(&provisionOverrides, "set", "s", []string{}, "set/override provisioning steps")
	execCmd.Flags().VarP(&containerPullPolicy, "container-pull-policy", "", fmt.Sprintf("Whether or not to pull container images used durign provisioning. Overrides the `pull` value of every provision step. Valid values: [%s, %s, %s]", pullpolicy.Always, pullpolicy.IfNotExist, pullpolicy.Never))

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

	containerCfg := containerapi.NewContainerConfig(
		"virter-"+strings.Join(vmNames, "-"),
		s.Image,
		s.Env,
		containerapi.WithCommand(s.Command...),
		containerapi.WithPullConfig(s.Pull.ForContainer()),
	)

	return v.VMExecDocker(ctx, containerProvider, vmNames, containerCfg, s.Copy)
}
