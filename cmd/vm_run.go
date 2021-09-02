package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rck/unit"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vbauerster/mpb/v7"
	"golang.org/x/sync/errgroup"

	"github.com/LINBIT/virter/internal/virter"
)

var sizeUnits = func() map[string]int64 {
	units := unit.DefaultUnits
	units["KiB"] = units["K"]
	units["MiB"] = units["M"]
	units["GiB"] = units["G"]
	units["TiB"] = units["T"]
	units["PiB"] = units["P"]
	units["EiB"] = units["E"]
	return units
}()

func createConsoleDir(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	// libvirt doesn't like relative paths
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to determine absolute path for console directory '%v': %w",
			path, err)
	}

	if err := os.MkdirAll(absPath, 0o700); err != nil {
		return "", fmt.Errorf("failed to create console directory at '%v': %w", absPath, err)
	}

	return absPath, nil
}

func createConsoleFile(consoleDir, vmName string) (string, error) {
	if consoleDir == "" {
		return "", nil
	}

	consolePath := filepath.Join(consoleDir, vmName+".log")

	file, err := os.Create(consolePath)
	if err != nil {
		return "", fmt.Errorf("failed to create console file at '%v': %w", consolePath, err)
	}
	file.Close()

	return consolePath, nil
}

func vmRunCommand() *cobra.Command {
	var vmName string
	var vmID uint
	var count uint
	var waitSSH bool

	var mem *unit.Value
	var memKiB uint64

	var bootCapacity *unit.Value
	var bootCapacityKiB uint64

	var vcpus uint
	cpuArch := virter.CpuArchNative

	var consoleDir string

	var gdbPort uint

	var diskStrings []string
	var disks []virter.Disk

	var nicStrings []string
	var nics []virter.NIC

	var provisionFile string
	var provisionOverrides []string

	pullPolicy := PullPolicyIfNotExist

	runCmd := &cobra.Command{
		Use:   "run image",
		Short: "Start a virtual machine with a given image",
		Long:  `Start a fresh virtual machine from an image.`,
		Args:  cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			memKiB = uint64(mem.Value / unit.DefaultUnits["K"])
			bootCapacityKiB = uint64(bootCapacity.Value / unit.DefaultUnits["K"])

			for _, s := range diskStrings {
				var d DiskArg
				err := d.Set(s)
				if err != nil {
					log.Fatalf("Invalid disk: %v", err)
				}
				disks = append(disks, &d)
			}

			for _, s := range nicStrings {
				var n NICArg
				err := n.Set(s)
				if err != nil {
					log.Fatalf("Invalid nic: %v", err)
				}
				nics = append(nics, &n)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
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

			p := mpb.New(DefaultContainerOpt())
			image, err := GetLocalImage(ctx, args[0], args[0], v, pullPolicy, DefaultProgressFormat(p))
			if err != nil {
				log.Fatalf("Error while getting image: %v", err)
			}

			p.Wait()

			consoleDir, err = createConsoleDir(consoleDir)
			if err != nil {
				log.Fatalf("Error while creating console directory: %v", err)
			}

			// do we want to run provisioning steps?
			provision := provisionFile != "" || len(provisionOverrides) > 0

			// if we want to run some provisioning steps later,
			// it doesn't make sense not to wait for SSH.
			if provision {
				waitSSH = true
			}

			var g errgroup.Group
			var i uint

			// save the VM names in case we want to provision later
			vmNames := make([]string, count)
			for i = 0; i < count; i++ {
				i := i
				id := vmID + i
				thisGDBPort := gdbPort
				if gdbPort != 0 && cmd.Flags().Changed("count") {
					thisGDBPort += id
				}
				g.Go(func() error {
					var thisVMName string
					if vmName == "" {
						// if the name is not set, use image name + id
						thisVMName = fmt.Sprintf("%s-%d", image.Name(), id)
					} else if !cmd.Flags().Changed("count") {
						// if it is set, use the supplied name if
						// --count is the default (1)
						thisVMName = vmName
					} else {
						// if the count is set explicitly, use the
						// supplied name and the id
						thisVMName = fmt.Sprintf("%s-%d", vmName, id)
					}
					vmNames[i] = thisVMName

					consolePath, err := createConsoleFile(consoleDir, thisVMName)
					if err != nil {
						log.Fatalf("Error while creating console file: %v", err)
					}

					c := virter.VMConfig{
						Image:              image,
						Name:               thisVMName,
						CpuArch:            cpuArch,
						MemoryKiB:          memKiB,
						BootCapacityKiB:    bootCapacityKiB,
						VCPUs:              vcpus,
						ID:                 id,
						StaticDHCP:         viper.GetBool("libvirt.static_dhcp"),
						ExtraSSHPublicKeys: extraAuthorizedKeys,
						ConsolePath:        consolePath,
						Disks:              disks,
						ExtraNics:          nics,
						GDBPort:            thisGDBPort,
					}

					err = v.VMRun(c)
					if err != nil {
						return fmt.Errorf("Failed to start VM %d: %w", id, err)
					}

					if waitSSH {
						sshPingConfig := virter.SSHPingConfig{
							SSHPingCount:  viper.GetInt("time.ssh_ping_count"),
							SSHPingPeriod: viper.GetDuration("time.ssh_ping_period"),
						}

						err = v.PingSSH(ctx, SSHClientBuilder{}, thisVMName, sshPingConfig)
						if err != nil {
							return fmt.Errorf("Failed to connect to VM %d over SSH: %w", id, err)
						}
					}
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				log.Fatal(err)
			}

			if provision {
				provOpt := virter.ProvisionOption{
					FilePath:  provisionFile,
					Overrides: provisionOverrides,
				}
				if err := execProvision(ctx, provOpt, vmNames); err != nil {
					log.Fatal(err)
				}
			}

			for _, name := range vmNames {
				fmt.Println(name)
			}
		},
	}

	runCmd.Flags().StringVarP(&vmName, "name", "n", "", "name of new VM")
	runCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID for VM which determines the IP address")
	runCmd.MarkFlagRequired("id")
	runCmd.Flags().UintVar(&count, "count", 1, "Number of VMs to start")
	runCmd.Flags().BoolVarP(&waitSSH, "wait-ssh", "w", false, "whether to wait for SSH port (default false)")
	u := unit.MustNewUnit(sizeUnits)
	mem = u.MustNewValue(1*sizeUnits["G"], unit.None)
	runCmd.Flags().VarP(mem, "memory", "m", "Set amount of memory for the VM")
	bootCapacity = u.MustNewValue(0, unit.None)
	runCmd.Flags().VarP(bootCapacity, "bootcapacity", "", "Capacity of the boot volume (default is the capacity of the base image, at least 10G)")
	runCmd.Flags().UintVar(&vcpus, "vcpus", 1, "Number of virtual CPUs to allocate for the VM")
	runCmd.Flags().VarP(&cpuArch, "arch", "", "CPU architecture to use. Will use kvm if host and VM use the same architecture")
	runCmd.Flags().StringVarP(&consoleDir, "console", "c", "", "Directory to save the VMs console outputs to")
	runCmd.Flags().UintVar(&gdbPort, "gdb-port", 0, "Enable gdb remote connection on this port (if --count is used, the ID will be added to this port number)")

	// Unfortunately, pflag cannot accept arrays of custom Values (yet?).
	// See https://github.com/spf13/pflag/issues/260
	// For us, this means that we have to read the disks as strings first,
	// and then manually marshal them to Disks.
	// If this ever gets implemented in pflag , we will be able to solve this
	// in a much smoother way.
	runCmd.Flags().StringArrayVarP(&diskStrings, "disk", "d", []string{}, `Add a disk to the VM. Format: "name=disk1,size=100MiB,format=qcow2,bus=virtio". Can be specified multiple times`)
	runCmd.Flags().StringArrayVarP(&nicStrings, "nic", "i", []string{}, `Add a NIC to the VM. Format: "type=network,source=some-net-name". Type can also be "bridge", in which case the source is the bridge device name. Additional config options are "model" (default: virtio) and "mac" (default chosen by libvirt). Can be specified multiple times`)

	runCmd.Flags().StringVarP(&provisionFile, "provision", "p", "", "name of toml file containing provisioning steps")
	runCmd.Flags().StringArrayVarP(&provisionOverrides, "set", "s", []string{}, "set/override provisioning steps")

	runCmd.Flags().VarP(&pullPolicy, "pull-policy", "", fmt.Sprintf("Whether or not to pull the source image. Valid values: [%s, %s, %s]", PullPolicyAlways, PullPolicyIfNotExist, PullPolicyNever))

	return runCmd
}
