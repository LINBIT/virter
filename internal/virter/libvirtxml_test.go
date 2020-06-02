package virter

import (
	"reflect"
	"testing"

	"github.com/kr/pretty"
	lx "github.com/libvirt/libvirt-go-xml"
)

func TestVmDisksToLibvirtDisks(t *testing.T) {
	cases := []struct {
		descr       string
		input       []VMDisk
		expect      []lx.DomainDisk
		expectError bool
	}{
		{
			descr: "one virtio, one ide",
			input: []VMDisk{
				VMDisk{device: VMDiskDeviceDisk, poolName: "pool", volumeName: "vol1", bus: "virtio", format: "qcow2"},
				VMDisk{device: VMDiskDeviceCDROM, poolName: "pool", volumeName: "vol2", bus: "ide", format: "raw"},
			},
			expect: []lx.DomainDisk{
				lx.DomainDisk{
					Device: "disk",
					Driver: &lx.DomainDiskDriver{
						Name:    "qemu",
						Discard: "unmap",
						Type:    "qcow2",
					},
					Source: &lx.DomainDiskSource{
						Volume: &lx.DomainDiskSourceVolume{
							Pool:   "pool",
							Volume: "vol1",
						},
					},
					Target: &lx.DomainDiskTarget{
						Dev: "vda",
						Bus: "virtio",
					},
				},
				lx.DomainDisk{
					Device: "cdrom",
					Driver: &lx.DomainDiskDriver{
						Name: "qemu",
						Type: "raw",
					},
					Source: &lx.DomainDiskSource{
						Volume: &lx.DomainDiskSourceVolume{
							Pool:   "pool",
							Volume: "vol2",
						},
					},
					Target: &lx.DomainDiskTarget{
						Dev: "hda",
						Bus: "ide",
					},
				},
			},
		}, {
			descr: "invalid bus",
			input: []VMDisk{
				VMDisk{device: VMDiskDeviceDisk, poolName: "pool", volumeName: "vol1", bus: "quaxi", format: "qcow2"},
			},
			expectError: true,
		},
	}

	for _, c := range cases {
		actual, err := vmDisksToLibvirtDisks(c.input)
		if !c.expectError && err != nil {
			t.Errorf("on input '%s':", c.input)
			t.Fatalf("unexpected error: %+v", err)
		}
		if c.expectError && err == nil {
			t.Errorf("on input '%s':", c.input)
			t.Fatalf("expected error, got nil")
		}
		if !reflect.DeepEqual(actual, c.expect) {
			t.Errorf("on input '%+v':", c.input)
			t.Errorf("unexpected result")
			pretty.Ldiff(t, c.expect, actual)
		}
	}
}
