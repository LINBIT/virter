package cmd

import (
	"context"
	"strings"

	log "github.com/sirupsen/logrus"

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
			provOpt := virter.ProvisionOption{
				FilePath:  provisionFile,
				Overrides: provisionOverrides,
			}
			if err := execProvision(provOpt, args); err != nil {
				log.Fatal(err)
			}
		},
	}

	execCmd.Flags().StringVarP(&provisionFile, "provision", "p", "", "name of toml file containing provisioning steps")
	execCmd.Flags().StringSliceVarP(&provisionOverrides, "set", "s", []string{}, "set/override provisioning steps")

	return execCmd
}

func execProvision(provOpt virter.ProvisionOption, vmNames []string) error {
	pc, err := virter.NewProvisionConfig(provOpt)
	if err != nil {
		return err
	}
	v, err := VirterConnect()
	if err != nil {
		log.Fatal(err)
	}
	defer v.Disconnect()

	for _, s := range pc.Steps {
		if s.Docker != nil {
			if err := execDocker(v, s.Docker, vmNames); err != nil {
				return err
			}
		} else if s.Shell != nil {
			if err := execShell(v, s.Shell, vmNames); err != nil {
				return err
			}
		} else if s.Rsync != nil {
			if err := execRsync(v, s.Rsync, vmNames); err != nil {
				return err
			}
		}
	}

	return nil
}

func execDocker(v *virter.Virter, s *virter.ProvisionDockerStep, vmNames []string) error {
	ctx, cancel := dockerContext()
	defer cancel()
	registerSignals(ctx, cancel)

	docker, err := dockerConnect()
	if err != nil {
		log.Fatal(err)
	}
	defer docker.Close()

	privateKey, err := loadPrivateKey()
	if err != nil {
		log.Fatal(err)
	}

	dockerContainerConfig := virter.DockerContainerConfig{
		ContainerName: "virter-" + strings.Join(vmNames, "-"),
		ImageName:     s.Image,
		Env:           virter.EnvmapToSlice(s.Env),
	}

	return v.VMExecDocker(ctx, docker, vmNames, dockerContainerConfig, privateKey)
}

func execShell(v *virter.Virter, s *virter.ProvisionShellStep, vmNames []string) error {
	privateKey, err := loadPrivateKey()
	if err != nil {
		log.Fatal(err)
	}

	return v.VMExecShell(context.TODO(), vmNames, privateKey, s)
}

func execRsync(v *virter.Virter, s *virter.ProvisionRsyncStep, vmNames []string) error {
	privateKeyPath := getPrivateKeyPath()
	copier := netcopy.NewRsyncNetworkCopier(privateKeyPath)
	return v.VMExecRsync(context.TODO(), copier, vmNames, s)
}
