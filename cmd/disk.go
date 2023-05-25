package cmd

import (
	"fmt"
	"github.com/LINBIT/virter/pkg/cliutils"
	"github.com/rck/unit"
)

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

// Set implements flag.Value.Set.
func (d *DiskArg) Set(str string) error {
	return cliutils.Parse(str, d)
}

// Type implements pflag.Value.Type.
func (d *DiskArg) Type() string { return "disk" }
