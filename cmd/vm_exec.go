package cmd

import (
	"context"
	"strings"

	"github.com/LINBIT/virter/internal/virter"
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func vmExecCommand() *cobra.Command {
	var provisionFile string

	execCmd := &cobra.Command{
		Use:   "exec vm_name [vm_name...]",
		Short: "Run a Docker container against a VM",
		Long:  `Run a Docker container on the host with a connection to a VM.`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := execProvision(provisionFile, args); err != nil {
				log.Fatal(err)
			}
		},
	}

	execCmd.Flags().StringVarP(&provisionFile, "provision", "p", "", "name of toml file containing provisioning steps")
	execCmd.MarkFlagRequired("provision")

	return execCmd
}

func execProvision(provisionFile string, vmNames []string) error {
	pc, err := virter.NewProvisionConfigFile(provisionFile)
	if err != nil {
		return err
	}

	for _, s := range pc.Steps {
		if s.Docker != nil {
			if err := execDocker(s.Docker, vmNames); err != nil {
				return err
			}
		} else if s.Shell != nil {
			if err := execShell(s.Shell, vmNames); err != nil {
				return err
			}
		}
	}

	return nil
}

func execDocker(s *virter.ProvisionDockerStep, vmNames []string) error {
	ctx, cancel := dockerContext()
	defer cancel()

	v, err := VirterConnect()
	if err != nil {
		log.Fatal(err)
	}

	docker, err := dockerConnect()
	if err != nil {
		log.Fatal(err)
	}

	privateKey, err := loadPrivateKey()
	if err != nil {
		log.Fatal(err)
	}

	dockerContainerConfig := virter.DockerContainerConfig{
		ContainerName: "virter-" + strings.Join(vmNames, "-"),
		ImageName:     s.Image,
		Env:           s.Env,
	}

	return v.VMExecDocker(ctx, docker, vmNames, dockerContainerConfig, privateKey)
}

func execShell(s *virter.ProvisionShellStep, vmNames []string) error {
	v, err := VirterConnect()
	if err != nil {
		log.Fatal(err)
	}

	privateKey, err := loadPrivateKey()
	if err != nil {
		log.Fatal(err)
	}

	return v.VMExecShell(context.TODO(), vmNames, privateKey, s)
}
