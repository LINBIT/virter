package cmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/LINBIT/containerapi"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
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
	var vmName string
	var provisionFile string
	var provisionOverrides []string

	var mem *unit.Value
	var memKiB uint64

	var bootCapacity *unit.Value
	var bootCapacityKiB uint64

	var vcpus uint

	var consoleDir string
	var resetMachineID bool

	var push bool
	var noCache bool
	var buildId string
	cpuArch := virter.CpuArchNative

	var mountStrings []string
	var mounts []virter.Mount

	pullPolicy := PullPolicyIfNotExist

	buildCmd := &cobra.Command{
		Use:   "build base_image new_image",
		Short: "Build an image",
		Long:  `Build an image by starting a VM, running a provisioning step, and then committing the resulting volume.`,
		Args:  cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			memKiB = uint64(mem.Value / unit.DefaultUnits["K"])
			bootCapacityKiB = uint64(bootCapacity.Value / unit.DefaultUnits["K"])

			for _, m := range mountStrings {
				var a MountArg
				err := a.Set(m)
				if err != nil {
					return fmt.Errorf("invalid mount: %w", err)
				}
				mounts = append(mounts, &a)
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			baseImageName := args[0]
			newImageName := LocalImageName(args[1])

			if vmName == "" {
				vmName = newImageName
			}

			ctx, cancel := onInterruptWrap(context.Background())
			defer cancel()

			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			var existingTargetImage regv1.Image
			var existingTargetRef name.Reference
			if push {
				existingTargetRef, err = name.ParseReference(args[1], name.WithDefaultRegistry(""))
				if err != nil {
					log.WithError(err).Fatal("failed to parse destination ref")
				}

				err = remote.CheckPushPermission(existingTargetRef, authn.DefaultKeychain, http.DefaultTransport)
				if err != nil {
					log.WithError(err).Fatal("not allowed to push")
				}

				// We deliberately ignore errors here, probably just tells us that the image doesn't exist yet.
				existingTargetImage, _ = remote.Image(existingTargetRef, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
			}

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

			p := mpb.NewWithContext(ctx, DefaultContainerOpt())

			baseImage, err := GetLocalImage(ctx, baseImageName, baseImageName, v, pullPolicy, DefaultProgressFormat(p))
			if err != nil {
				log.Fatalf("Error while getting image: %v", err)
			}

			p.Wait()

			provOpt := virter.ProvisionOption{
				FilePath:  provisionFile,
				Overrides: provisionOverrides,
			}

			provisionConfig, err := virter.NewProvisionConfig(provOpt)
			if err != nil {
				log.Fatal(err)
			}

			if push && buildId == "" {
				log.Info("Pushing without providing a build ID. Images will always be rebuilt unless the same build ID is given.")
			}

			if buildId != "" && existingTargetImage != nil && !noCache {
				unchanged, err := provisionStepsUnchanged(baseImage, existingTargetImage, buildId)
				if err != nil {
					log.WithError(err).Warn("error comparing existing target image, assuming provision steps changed")
				} else if unchanged {
					log.Info("Image already up-to-date, skipping provision, pulling instead")

					p := mpb.NewWithContext(ctx, DefaultContainerOpt())

					_, err := GetLocalImage(ctx, newImageName, args[1], v, PullPolicyAlways, DefaultProgressFormat(p))
					if err != nil {
						log.Fatal(err)
					}

					p.Wait()

					fmt.Printf("Built %s\n", newImageName)
					return
				}
			}

			// ContainerProvider will be set later if needed
			tools := virter.ImageBuildTools{
				ShellClientBuilder: SSHClientBuilder{},
				AfterNotifier:      actualtime.ActualTime{},
			}

			vmConfig := virter.VMConfig{
				Image:              baseImage,
				Name:               vmName,
				CpuArch:            cpuArch,
				MemoryKiB:          memKiB,
				BootCapacityKiB:    bootCapacityKiB,
				VCPUs:              vcpus,
				ID:                 vmID,
				StaticDHCP:         viper.GetBool("libvirt.static_dhcp"),
				ExtraSSHPublicKeys: extraAuthorizedKeys,
				ConsolePath:        consolePath,
				Mounts:             mounts,
			}

			sshPingConfig := virter.SSHPingConfig{
				SSHPingCount:  viper.GetInt("time.ssh_ping_count"),
				SSHPingPeriod: viper.GetDuration("time.ssh_ping_period"),
			}

			containerName := "virter-build-" + newImageName

			if provisionConfig.NeedsContainers() {
				containerProvider, err := containerapi.NewProvider(ctx, containerProvider())
				if err != nil {
					log.Fatal(err)
				}
				defer containerProvider.Close()
				tools.ContainerProvider = containerProvider
			}

			buildConfig := virter.ImageBuildConfig{
				ImageName:       newImageName,
				ContainerName:   containerName,
				ShutdownTimeout: shutdownTimeout,
				ProvisionConfig: provisionConfig,
				ResetMachineID:  resetMachineID,
			}

			p = mpb.NewWithContext(ctx, DefaultContainerOpt())

			err = v.ImageBuild(ctx, tools, vmConfig, sshPingConfig, buildConfig, virter.WithProgress(DefaultProgressFormat(p)))
			if err != nil {
				log.Fatalf("Failed to build image: %v", err)
			}

			if push {
				localImg, err := v.FindImage(newImageName, virter.WithProgress(DefaultProgressFormat(p)))
				if err != nil {
					log.Fatalf("failed to find built image: %v", err)
				}

				if localImg == nil {
					log.Fatal("failed to find built image: not found")
				}

				imageWithHistory := &historyShimImage{
					Image: localImg,
					history: []regv1.History{
						{Comment: buildId},
					},
				}

				err = remote.Write(existingTargetRef, imageWithHistory, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
				if err != nil {
					log.Fatalf("failed to push image: %v", err)
				}
			}

			p.Wait()

			fmt.Printf("Built %s\n", newImageName)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) < 2 {
				// Only suggest first and second argument
				return suggestImageNames(cmd, args, toComplete)
			}

			return suggestNone(cmd, args, toComplete)
		},
	}

	buildCmd.Flags().StringVarP(&provisionFile, "provision", "p", "", "name of toml file containing provisioning steps")
	buildCmd.Flags().StringArrayVarP(&provisionOverrides, "set", "s", []string{}, "set/override provisioning steps")
	buildCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID for VM which determines the IP address")
	buildCmd.Flags().StringVarP(&vmName, "name", "", "", "Name to use for provisioning VM")
	buildCmd.Flags().UintVar(&vcpus, "vcpus", 1, "Number of virtual CPUs to allocate for the VM")
	buildCmd.Flags().VarP(&cpuArch, "arch", "", "CPU architecture to use. Will use kvm if host and VM use the same architecture")
	u := unit.MustNewUnit(sizeUnits)
	mem = u.MustNewValue(1*sizeUnits["G"], unit.None)
	buildCmd.Flags().VarP(mem, "memory", "m", "Set amount of memory for the VM")
	bootCapacity = u.MustNewValue(10*sizeUnits["G"], unit.None)
	buildCmd.Flags().VarP(bootCapacity, "bootcap", "", "Capacity of the boot volume (values smaller than base image capacity will be ignored)")
	buildCmd.Flags().StringVarP(&consoleDir, "console", "c", "", "Directory to save the VMs console outputs to")
	buildCmd.Flags().BoolVar(&resetMachineID, "reset-machine-id", true, "Whether or not to clear the /etc/machine-id file after provisioning")
	buildCmd.Flags().VarP(&pullPolicy, "pull-policy", "", fmt.Sprintf("Whether or not to pull the source image. Valid values: [%s, %s, %s]", PullPolicyAlways, PullPolicyIfNotExist, PullPolicyNever))
	buildCmd.Flags().BoolVarP(&push, "push", "", false, "Push the image after building")
	buildCmd.Flags().BoolVarP(&noCache, "no-cache", "", false, "Disable caching for the image build")
	buildCmd.Flags().StringVarP(&buildId, "build-id", "", "", "Build ID used to determine if an image needs to be rebuild.")
	buildCmd.Flags().StringArrayVarP(&mountStrings, "mount", "v", []string{}, `Mount a host path in the VM, like a bind mount. Format: "host=/path/on/host,vm=/path/in/vm"`)

	return buildCmd
}

// historyShimImage adds history to an existing image
type historyShimImage struct {
	regv1.Image
	history []regv1.History
}

func (h *historyShimImage) Size() (int64, error) {
	return partial.Size(h)
}

func (h *historyShimImage) ConfigName() (regv1.Hash, error) {
	return partial.ConfigName(h)
}

func (h *historyShimImage) ConfigFile() (*regv1.ConfigFile, error) {
	original, err := h.Image.ConfigFile()
	if err != nil {
		return nil, err
	}

	original.History = h.history
	return original, err
}

func (h *historyShimImage) RawConfigFile() ([]byte, error) {
	return partial.RawConfigFile(h)
}

func (h *historyShimImage) Digest() (regv1.Hash, error) {
	return partial.Digest(h)
}

func (h *historyShimImage) Manifest() (*regv1.Manifest, error) {
	original, err := h.Image.Manifest()
	if err != nil {
		return nil, err
	}

	raw, err := h.RawConfigFile()
	if err != nil {
		return nil, err
	}

	cfgHash, cfgSize, err := regv1.SHA256(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}

	original.Config = regv1.Descriptor{
		MediaType: virter.ImageMediaType,
		Size:      cfgSize,
		Digest:    cfgHash,
	}

	return original, nil
}

func (h *historyShimImage) RawManifest() ([]byte, error) {
	return partial.RawManifest(h)
}

var _ regv1.Image = &historyShimImage{}

func provisionStepsUnchanged(baseImage *virter.LocalImage, targetImage regv1.Image, expectedHistory string) (bool, error) {
	targetCfg, err := targetImage.ConfigFile()
	if err != nil {
		return false, err
	}

	if len(targetCfg.History) == 0 {
		// No history information, image wasn't provision with (new) virter
		return false, nil
	}

	lastHistoryEntry := targetCfg.History[len(targetCfg.History)-1]
	if string(expectedHistory) != lastHistoryEntry.Comment {
		return false, nil
	}

	if len(targetCfg.RootFS.DiffIDs) < 2 {
		// There doesn't seem to be a base layer for this image
		return false, nil
	}

	targetBaseImageID := targetCfg.RootFS.DiffIDs[len(targetCfg.RootFS.DiffIDs)-2]

	currentBaseImageID, err := baseImage.TopLayer().DiffID()
	if err != nil {
		return false, err
	}

	return targetBaseImageID == currentBaseImageID, nil
}
