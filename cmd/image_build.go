package cmd

import (
	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/actualtime"
	"github.com/LINBIT/virter/pkg/isogenerator"
)

func imageBuildCommand() *cobra.Command {
	var vmID uint
	var provisionFile string

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
				MemoryKiB:     1048576, // one gigabyte
				VCPUs:         1,
				VMID:          vmID,
				SSHPublicKeys: publicKeys,
			}

			dockerContainerConfig := virter.DockerContainerConfig{
				ContainerName: "virter-build-" + newImageName,
			}

			var provisionConfig virter.ProvisionConfig
			_, err = toml.DecodeFile(provisionFile, &provisionConfig)
			if err != nil {
				log.Fatal(err)
			}

			buildConfig := virter.ImageBuildConfig{
				DockerContainerConfig: dockerContainerConfig,
				SSHPrivateKey:         privateKey,
				ShutdownTimeout:       shutdownTimeout,
				ProvisionConfig:       provisionConfig,
			}

			err = v.ImageBuild(ctx, tools, vmConfig, buildConfig)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	buildCmd.Flags().StringVarP(&provisionFile, "provision", "p", "", "name of toml file containing provisioning steps")
	buildCmd.MarkFlagRequired("provision")
	buildCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID for VM which determines the IP address")
	buildCmd.MarkFlagRequired("id")

	return buildCmd
}
