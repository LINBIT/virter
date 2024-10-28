package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"

	log "github.com/sirupsen/logrus"

	"github.com/LINBIT/containerapi"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/netcopy"
	"github.com/LINBIT/virter/pkg/pullpolicy"

	"github.com/spf13/cobra"
)

// logProvisioningErrorAndExit logs an error from a virter.VMExec* function and exits with the appropriate exit code.
// If the error is from a failed SSH or container provisioning step, the exit code is the exit code
// of the respective command.
// Otherwise, the exit code is 1.
func logProvisioningErrorAndExit(err error) {
	log.Errorf("Failed to build image: %v", err)
	var sshErr *ssh.ExitError
	if errors.As(err, &sshErr) {
		os.Exit(sshErr.ExitStatus())
	}
	var containerErr *virter.ContainerExitError
	if errors.As(err, &containerErr) {
		os.Exit(containerErr.Status)
	}
	os.Exit(1)
}

func vmExecCommand() *cobra.Command {
	var provFile FileVar
	var provisionOverrides []string

	var containerPullPolicy pullpolicy.PullPolicy

	execCmd := &cobra.Command{
		Use:   "exec vm_name [vm_name...]",
		Short: "Run provisioning steps on VMs",
		Long:  `Run provisioning steps. For instance, shell scripts directly on VMs, or from a container with connections to VMs.`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			provOpt := virter.ProvisionOption{
				Overrides:          provisionOverrides,
				DefaultPullPolicy:  getDefaultContainerPullPolicy(),
				OverridePullPolicy: containerPullPolicy,
			}
			if err := execProvision(cmd.Context(), provFile.File, provOpt, args); err != nil {
				logProvisioningErrorAndExit(err)
			}
		},
		ValidArgsFunction: suggestVmNames,
	}

	execCmd.Flags().VarP(&provFile, "provision", "p", "name of toml file containing provisioning steps")
	execCmd.Flags().StringArrayVarP(&provisionOverrides, "set", "s", []string{}, "set/override provisioning steps")
	execCmd.Flags().VarP(&containerPullPolicy, "container-pull-policy", "", fmt.Sprintf("Whether or not to pull container images used during provisioning. Overrides the `pull` value of every provision step. Valid values: [%s, %s, %s]", pullpolicy.Always, pullpolicy.IfNotExist, pullpolicy.Never))

	return execCmd
}

func execProvision(ctx context.Context, provFileReader io.ReadCloser, provOpt virter.ProvisionOption, vmNames []string) error {
	pc, err := virter.NewProvisionConfig(provFileReader, provOpt)
	if err != nil {
		return err
	}
	v, err := InitVirter()
	if err != nil {
		log.Fatal(err)
	}
	defer v.ForceDisconnect()

	for _, s := range pc.Steps {
		if s.Container != nil {
			if err := execContainer(ctx, v, s.Container, vmNames); err != nil {
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

func execContainer(ctx context.Context, v *virter.Virter, s *virter.ProvisionContainerStep, vmNames []string) error {
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

	return v.VMExecContainer(ctx, containerProvider, vmNames, containerCfg, s.Copy)
}
