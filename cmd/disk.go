package cmd

import (
	"fmt"

	"github.com/rck/unit"
	"github.com/spf13/cobra"

	"github.com/LINBIT/virter/pkg/cliutils"
)

func diskCommand() *cobra.Command {
	diskCmd := &cobra.Command{
		Use:   "disk",
		Short: "Shared disk related subcommands",
		Long: `Shared disk related subcommands. Shared disks can be attached to multiple
VMs at the same time, for example to simulate a SAN attached volume.`,
	}

	diskCmd.AddCommand(diskCreateCommand())
	diskCmd.AddCommand(diskLsCommand())
	diskCmd.AddCommand(diskRmCommand())
	return diskCmd
}

// DiskArg represents a disk that can be passed to virter via a command line argument.
type DiskArg struct {
	Name   string `arg:"name"`
	Size   Size   `arg:"size"`
	Format string `arg:"format,qcow2"`
	Bus    string `arg:"bus,virtio"`
	Pool   string `arg:"pool,"`
}

type Size struct {
	KiB uint64
}

func (s *Size) UnmarshalText(text []byte) error {
	u := unit.MustNewUnit(sizeUnits)
	val, err := u.ValueFromString(string(text))
	if err != nil {
		return fmt.Errorf("invalid size: %w", err)
	}
	signedSizeKiB := val.Value / sizeUnits["K"]
	if signedSizeKiB < 0 {
		return fmt.Errorf("invalid size: must be positive number")
	}
	s.KiB = uint64(signedSizeKiB)
	return nil
}

func (d *DiskArg) GetName() string    { return d.Name }
func (d *DiskArg) GetSizeKiB() uint64 { return d.Size.KiB }
func (d *DiskArg) GetFormat() string  { return d.Format }
func (d *DiskArg) GetBus() string     { return d.Bus }
func (d *DiskArg) GetPool() string    { return d.Pool }
func (d *DiskArg) GetShareable() bool { return false }

// Set implements flag.Value.Set.
func (d *DiskArg) Set(str string) error {
	return cliutils.Parse(str, d)
}

// Type implements pflag.Value.Type.
func (d *DiskArg) Type() string { return "disk" }

// SharedDiskArg represents a reference to an existing shared disk that can be
// passed to virter via a command line argument.
type SharedDiskArg struct {
	Name string `arg:"name"`
	Bus  string `arg:"bus,virtio"`
	Pool string `arg:"pool,"`
}

func (d *SharedDiskArg) GetName() string    { return d.Name }
func (d *SharedDiskArg) GetSizeKiB() uint64 { return 0 }
func (d *SharedDiskArg) GetFormat() string  { return "raw" }
func (d *SharedDiskArg) GetBus() string     { return d.Bus }
func (d *SharedDiskArg) GetPool() string    { return d.Pool }
func (d *SharedDiskArg) GetShareable() bool { return true }

// Set implements flag.Value.Set.
func (d *SharedDiskArg) Set(str string) error {
	return cliutils.Parse(str, d)
}

// Type implements pflag.Value.Type.
func (d *SharedDiskArg) Type() string { return "shared-disk" }
