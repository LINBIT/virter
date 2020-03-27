package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/actualtime"
	"github.com/LINBIT/virter/pkg/isogenerator"
)

func imageBuildCommand() *cobra.Command {
	var dockerEnv []string
	var dockerImageName string
	var vmID uint

	buildCmd := &cobra.Command{
		Use:   "build base_image new_image",
		Short: "Build an image",
		Long: `Build an image by starting a VM, running a provisioning
step, and then committing the resulting volume.`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			baseImageName := args[0]
			newImageName := args[1]

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

			publicKeys, err := loadPublicKeys()
			if err != nil {
				log.Fatal(err)
			}

			privateKey, err := loadPrivateKey()
			if err != nil {
				log.Fatal(err)
			}

			shutdownTimeout := viper.GetDuration("time.shutdown_timeout")

			tools := virter.ImageBuildTools{
				ISOGenerator:  isogenerator.ExternalISOGenerator{},
				PortWaiter:    newSSHPinger(),
				DockerClient:  docker,
				AfterNotifier: actualtime.ActualTime{},
			}

			vmConfig := virter.VMConfig{
				ImageName:     baseImageName,
				VMName:        newImageName,
				VMID:          vmID,
				SSHPublicKeys: publicKeys,
			}

			dockerContainerConfig := virter.DockerContainerConfig{
				ImageName: dockerImageName,
				Env:       dockerEnv,
			}
			buildConfig := virter.ImageBuildConfig{
				DockerContainerConfig: dockerContainerConfig,
				SSHPrivateKey:         privateKey,
				ShutdownTimeout:       shutdownTimeout,
			}

			err = v.ImageBuild(ctx, tools, vmConfig, buildConfig)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	buildCmd.Flags().StringArrayVarP(&dockerEnv, "env", "e", []string{}, "environment variables to pass to the container (e.g., FOO=bar)")
	buildCmd.Flags().StringVarP(&dockerImageName, "docker-image", "d", "", "name of Docker image to run")
	buildCmd.MarkFlagRequired("docker-image")
	buildCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID for VM which determines the IP address")
	buildCmd.MarkFlagRequired("id")

	return buildCmd
}
