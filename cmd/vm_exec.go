package cmd

import (
	"context"
	"io/ioutil"
	"log"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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

			dockerTimeout := viper.GetDuration("time.docker_timeout")
			ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
			defer cancel()

			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}

			docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				log.Fatalf("could not connect to Docker %v", err)
			}

			privateKeyPath := viper.GetString("auth.virter_private_key_path")
			privateKey, err := ioutil.ReadFile(privateKeyPath)
			if err != nil {
				log.Fatalf("failed to load private key from '%s': %v", privateKeyPath, err)
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
