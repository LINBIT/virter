package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"

	"github.com/LINBIT/virter/internal/virter"
)

func vmExecCommand() *cobra.Command {
	var dockerEnv []string
	var dockerImageName string

	execCmd := &cobra.Command{
		Use:   "exec vm_name",
		Short: "Run a Docker container against a VM",
		Long:  `Run a Docker container on the host with a connection to a VM.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			vmName := args[0]

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
				ImageName: dockerImageName,
				Env:       dockerEnv,
			}
			err = v.VMExec(ctx, docker, vmName, dockerContainerConfig, privateKey)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	execCmd.Flags().StringArrayVarP(&dockerEnv, "env", "e", []string{}, "environment variables to pass to the container (e.g., FOO=bar)")
	execCmd.Flags().StringVarP(&dockerImageName, "docker-image", "d", "", "name of Docker image to run")
	execCmd.MarkFlagRequired("docker-image")

	return execCmd
}
