package cmd

import (
	"errors"
	"fmt"
	"os/user"
	"path/filepath"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/rck/unit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/registry"
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

// currentUidGid returns the user id and group id of the current user, parsed
// as an uint32. An error is returned if the retrieval of the user or parsing
// of the IDs fails.
func currentUidGid() (uint32, uint32, error) {
	u, err := user.Current()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get current user: %w", err)
	}

	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to convert uid '%s' to number: %w",
			u.Uid, err)
	}

	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to convert gid '%s' to number: %w",
			u.Gid, err)
	}

	// uid and gid are uint64, but we can safely cast here because we
	// ensured bitsize = 32 in the ParseUint calls above
	return uint32(uid), uint32(gid), nil
}

func currentUserConsoleFile(filename string) (*virter.VMConsoleFile, error) {
	if filename == "" {
		return nil, nil
	}
	currentUser, currentGroup, err := currentUidGid()
	if err != nil {
		log.Warnf("Failed to determine current user: %v", err)
		log.Warnf("Creating console logfile as root")
		currentUser, currentGroup = 0, 0
	}

	// libvirt doesn't like relative paths
	path, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to determine absolute path for console file '%v': %w",
			filename, err)
	}

	return &virter.VMConsoleFile{
		Path:     path,
		OwnerUID: currentUser,
		OwnerGID: currentGroup,
	}, nil
}

func pullIfNotExists(v *virter.Virter, imageName string) error {
	exists, err := v.ImageExists(imageName)
	if err != nil {
		return fmt.Errorf("could not determine whether or not image %v exists: %w",
			imageName, err)
	}
	if !exists {
		log.Printf("Image %v not available locally, pulling", imageName)
		e := pullImage(v, imageName, "")
		if errors.Is(e, registry.ErrNotFound) {
			return fmt.Errorf("Could not find image %v", imageName)
		} else if e != nil {
			return fmt.Errorf("Error pulling image %v: %w", imageName, e)
		}
	}

	return nil
}

func vmRunCommand() *cobra.Command {
	var vmName string
	var vmID uint
	var waitSSH bool

	var mem *unit.Value
	var memKiB uint64

	var vcpus uint

	var consoleFile string

	var diskStrings []string
	var disks []virter.Disk

	runCmd := &cobra.Command{
		Use:   "run image",
		Short: "Start a virtual machine with a given image",
		Long:  `Start a fresh virtual machine from an image.`,
		Args:  cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			memKiB = uint64(mem.Value / unit.DefaultUnits["K"])

			for _, s := range diskStrings {
				var d DiskArg
				err := d.Set(s)
				if err != nil {
					log.Fatalf("Invalid disk: %v", err)
				}
				disks = append(disks, &d)
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}
			defer v.Disconnect()

			imageName := args[0]
			if vmName == "" {
				vmName = fmt.Sprintf("%s-%d", imageName, vmID)
			}

			publicKeys, err := loadPublicKeys()
			if err != nil {
				log.Fatal(err)
			}

			privateKey, err := loadPrivateKey()
			if err != nil {
				log.Fatal(err)
			}

			console, err := currentUserConsoleFile(consoleFile)
			if err != nil {
				log.Fatalf("Error while configuring console: %v", err)
			}
			c := virter.VMConfig{
				ImageName:     imageName,
				Name:          vmName,
				MemoryKiB:     memKiB,
				VCPUs:         vcpus,
				ID:            vmID,
				SSHPublicKeys: publicKeys,
				SSHPrivateKey: privateKey,
				WaitSSH:       waitSSH,
				SSHPingCount:  viper.GetInt("time.ssh_ping_count"),
				SSHPingPeriod: viper.GetDuration("time.ssh_ping_period"),
				ConsoleFile:   console,
				Disks:         disks,
			}

			err = pullIfNotExists(v, c.ImageName)
			if err != nil {
				log.Fatal(err)
			}

			err = v.VMRun(SSHClientBuilder{}, c)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	runCmd.Flags().StringVarP(&vmName, "name", "n", "", "name of new VM")
	runCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID for VM which determines the IP address")
	runCmd.MarkFlagRequired("id")
	runCmd.Flags().BoolVarP(&waitSSH, "wait-ssh", "w", false, "whether to wait for SSH port (default false)")
	u := unit.MustNewUnit(sizeUnits)
	mem = u.MustNewValue(1*sizeUnits["G"], unit.None)
	runCmd.Flags().VarP(mem, "memory", "m", "Set amount of memory for the VM")
	runCmd.Flags().UintVar(&vcpus, "vcpus", 1, "Number of virtual CPUs to allocate for the VM")
	runCmd.Flags().StringVarP(&consoleFile, "console", "c", "", "File to redirect the VM's console output to")

	// Unfortunately, pflag cannot accept arrays of custom Values (yet?).
	// See https://github.com/spf13/pflag/issues/260
	// For us, this means that we have to read the disks as strings first,
	// and then manually marshal them to Disks.
	// If this ever gets implemented in pflag , we will be able to solve this
	// in a much smoother way.
	runCmd.Flags().StringArrayVarP(&diskStrings, "disk", "d", []string{}, `Add a disk to the VM. Format: "name=disk1,size=100MiB,format=qcow2,bus=virtio". Can be specified multiple times`)

	return runCmd
}
