package cmd

import (
	"context"
	"fmt"

	"github.com/LINBIT/containerapi"
	"github.com/rck/unit"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vbauerster/mpb/v7"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/actualtime"
)

func imageBuildCommand() *cobra.Command {
	var vmID uint
	var provisionFile string
	var provisionOverrides []string

	var mem *unit.Value
	var memKiB uint64

	var bootCapacity *unit.Value
	var bootCapacityKiB uint64

	var vcpus uint

	var consoleDir string
	var resetMachineID bool

	buildCmd := &cobra.Command{
		Use:   "build base_image new_image",
		Short: "Build an image",
		Long: `Build an image by starting a VM, running a provisioning
step, and then committing the resulting volume.`,
		Args: cobra.ExactArgs(2),
		PreRun: func(cmd *cobra.Command, args []string) {
			memKiB = uint64(mem.Value / unit.DefaultUnits["K"])
			bootCapacityKiB = uint64(bootCapacity.Value / unit.DefaultUnits["K"])
		},
		Run: func(cmd *cobra.Command, args []string) {
			baseImageName := args[0]
			newImageName := args[1]

			ctx, cancel := onInterruptWrap(context.Background())
			defer cancel()

			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			extraAuthorizedKeys, err := extraAuthorizedKeys()
			if err != nil {
				log.Fatal(err)
			}

			consoleDir, err = createConsoleDir(consoleDir)
			if err != nil {
				log.Fatalf("Error while creating console directory: %v", err)
			}

			consolePath, err := createConsoleFile(consoleDir, newImageName)
			if err != nil {
				log.Fatalf("Error while creating console file: %v", err)
			}

			shutdownTimeout := viper.GetDuration("time.shutdown_timeout")

			baseImage, err := v.FindImage(baseImageName)
			if err != nil {
				log.Fatalf("Error while getting image: %v", err)
			}

			p := mpb.NewWithContext(ctx)

			if baseImage == nil {
				// Try the "legacy" registry
				reg := loadRegistry()
				url, err := reg.Lookup(baseImageName)
				if err != nil {
					log.WithError(err).Fatal("unknown image")
				}

				baseImage, err = pullLegacyRegistry(ctx, v, baseImageName, url, p)
				if err != nil {
					log.WithError(err).Fatal("failed to pull image")
				}

				p.Wait()
			}

			// ContainerProvider will be set later if needed
			tools := virter.ImageBuildTools{
				ShellClientBuilder: SSHClientBuilder{},
				AfterNotifier:      actualtime.ActualTime{},
			}

			vmConfig := virter.VMConfig{
				Image:              baseImage,
				Name:               newImageName,
				MemoryKiB:          memKiB,
				BootCapacityKiB:    bootCapacityKiB,
				VCPUs:              vcpus,
				ID:                 vmID,
				StaticDHCP:         viper.GetBool("libvirt.static_dhcp"),
				ExtraSSHPublicKeys: extraAuthorizedKeys,
				ConsolePath:        consolePath,
			}

			sshPingConfig := virter.SSHPingConfig{
				SSHPingCount:  viper.GetInt("time.ssh_ping_count"),
				SSHPingPeriod: viper.GetDuration("time.ssh_ping_period"),
			}

			containerName := "virter-build-" + newImageName

			provOpt := virter.ProvisionOption{
				FilePath:  provisionFile,
				Overrides: provisionOverrides,
			}

			provisionConfig, err := virter.NewProvisionConfig(provOpt)
			if err != nil {
				log.Fatal(err)
			}
			if provisionConfig.NeedsContainers() {
				containerProvider, err := containerapi.NewProvider(ctx, containerProvider())
				if err != nil {
					log.Fatal(err)
				}
				defer containerProvider.Close()
				tools.ContainerProvider = containerProvider
			}

			buildConfig := virter.ImageBuildConfig{
				ContainerName:   containerName,
				ShutdownTimeout: shutdownTimeout,
				ProvisionConfig: provisionConfig,
				ResetMachineID:  resetMachineID,
			}

			err = v.ImageBuild(ctx, tools, vmConfig, sshPingConfig, buildConfig, virter.WithProgress(DefaultProgressFormat(p)))
			if err != nil {
				log.Fatalf("Failed to build image: %v", err)
			}

			p.Wait()

			fmt.Printf("Built %s\n", newImageName)
		},
	}

	buildCmd.Flags().StringVarP(&provisionFile, "provision", "p", "", "name of toml file containing provisioning steps")
	buildCmd.Flags().StringArrayVarP(&provisionOverrides, "set", "s", []string{}, "set/override provisioning steps")
	buildCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID for VM which determines the IP address")
	buildCmd.Flags().UintVar(&vcpus, "vcpus", 1, "Number of virtual CPUs to allocate for the VM")
	u := unit.MustNewUnit(sizeUnits)
	mem = u.MustNewValue(1*sizeUnits["G"], unit.None)
	buildCmd.Flags().VarP(mem, "memory", "m", "Set amount of memory for the VM")
	bootCapacity = u.MustNewValue(0, unit.None)
	buildCmd.Flags().VarP(bootCapacity, "bootcap", "", "Capacity of the boot volume (default is the capacity of the base image, at least 10G)")
	buildCmd.Flags().StringVarP(&consoleDir, "console", "c", "", "Directory to save the VMs console outputs to")
	buildCmd.Flags().BoolVar(&resetMachineID, "reset-machine-id", true, "Whether or not to clear the /etc/machine-id file after provisioning")

	return buildCmd
}
