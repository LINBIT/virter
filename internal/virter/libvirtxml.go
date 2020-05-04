package virter

import (
	"fmt"

	lx "github.com/libvirt/libvirt-go-xml"
	log "github.com/sirupsen/logrus"
)

func (v *Virter) vmXML(poolName string, vm VMConfig, mac string) (string, error) {
	domain := &lx.Domain{
		Type: "kvm",
		Name: vm.Name,
		Memory: &lx.DomainMemory{
			Unit: "KiB",
			// NOTE: because we cast to uint here, and we always
			// supply the memory in KiB, this will not support
			// memory sizes over 4TiB.
			Value: uint(vm.MemoryKiB),
		},
		VCPU: &lx.DomainVCPU{
			Placement: "static",
			Value:     int(vm.VCPUs),
		},
		OS: &lx.DomainOS{
			Type: &lx.DomainOSType{
				Arch: "x86_64",
				Type: "hvm",
			},
		},
		Features: &lx.DomainFeatureList{
			ACPI: &lx.DomainFeature{},
			APIC: &lx.DomainFeatureAPIC{},
		},
		CPU: &lx.DomainCPU{
			Mode: "host-passthrough",
		},
		Clock: &lx.DomainClock{
			Offset: "utc",
			Timer: []lx.DomainTimer{
				lx.DomainTimer{Name: "rtc", TickPolicy: "catchup"},
				lx.DomainTimer{Name: "pit", TickPolicy: "delay"},
				lx.DomainTimer{Name: "hpet", Present: "no"},
			},
		},
		OnPoweroff: "destroy",
		OnReboot:   "restart",
		OnCrash:    "destroy",
		PM: &lx.DomainPM{
			SuspendToMem:  &lx.DomainPMPolicy{Enabled: "no"},
			SuspendToDisk: &lx.DomainPMPolicy{Enabled: "no"},
		},
		Devices: &lx.DomainDeviceList{
			Disks: []lx.DomainDisk{
				libvirtDisk(poolName, vm.Name, "virtio", "vda"),
				libvirtCDROM(poolName, ciDataVolumeName(vm.Name), "ide", "hda"),
				libvirtDisk(poolName, scratchVolumeName(vm.Name), "scsi", "sda"),
			},
			Controllers: []lx.DomainController{
				lx.DomainController{
					Type:  "scsi",
					Model: "virtio-scsi",
				},
			},
			Interfaces: []lx.DomainInterface{
				lx.DomainInterface{
					MAC: &lx.DomainInterfaceMAC{
						Address: mac,
					},
					Source: &lx.DomainInterfaceSource{
						Network: &lx.DomainInterfaceSourceNetwork{
							Network: v.networkName,
							Bridge:  "virbr0",
						},
					},
					Model: &lx.DomainInterfaceModel{
						Type: "virtio",
					},
				},
			},
			Consoles: []lx.DomainConsole{
				libvirtConsole(vm.ConsoleFile),
			},
			Videos: []lx.DomainVideo{
				lx.DomainVideo{
					Model: lx.DomainVideoModel{
						Type: "cirrus",
					},
				},
			},
			MemBalloon: &lx.DomainMemBalloon{
				Model: "virtio",
				Alias: &lx.DomainAlias{
					Name: "ballon0",
				},
			},
		},
	}
	return domain.Marshal()
}

func (v *Virter) ciDataVolumeXML(name string) (string, error) {
	volume := &lx.StorageVolume{
		Name:     name,
		Capacity: &lx.StorageVolumeSize{Value: 0, Unit: "B"},
		Target: &lx.StorageVolumeTarget{
			Format: &lx.StorageVolumeTargetFormat{Type: "raw"},
		},
	}
	return volume.Marshal()
}

func (v *Virter) vmVolumeXML(name string, backingPath string) (string, error) {
	volume := &lx.StorageVolume{
		Name:     name,
		Capacity: &lx.StorageVolumeSize{Value: 10, Unit: "GiB"},
		Target: &lx.StorageVolumeTarget{
			Format: &lx.StorageVolumeTargetFormat{Type: "qcow2"},
		},
		BackingStore: &lx.StorageVolumeBackingStore{
			Path:   backingPath,
			Format: &lx.StorageVolumeTargetFormat{Type: "qcow2"},
		},
	}
	return volume.Marshal()
}

func (v *Virter) scratchVolumeXML(name string) (string, error) {
	volume := &lx.StorageVolume{
		Name:     name,
		Capacity: &lx.StorageVolumeSize{Value: 2, Unit: "GiB"},
		Target: &lx.StorageVolumeTarget{
			Format: &lx.StorageVolumeTargetFormat{Type: "qcow2"},
		},
	}
	return volume.Marshal()
}

func (v *Virter) imageVolumeXML(name string) (string, error) {
	volume := &lx.StorageVolume{
		Name:     name,
		Capacity: &lx.StorageVolumeSize{Value: 0, Unit: "B"},
		Target: &lx.StorageVolumeTarget{
			Format: &lx.StorageVolumeTargetFormat{Type: "qcow2"},
		},
	}
	return volume.Marshal()
}

func libvirtConsole(file *VMConsoleFile) lx.DomainConsole {
	var source *lx.DomainChardevSource
	// no file -> just return a regular PTY console
	if file == nil {
		source = &lx.DomainChardevSource{
			Pty: &lx.DomainChardevSourcePty{},
		}
	} else {
		log.Debugf("Logging VM console output to %s", file.Path)

		source = &lx.DomainChardevSource{
			File: &lx.DomainChardevSourceFile{
				Path:   file.Path,
				Append: "off",
				SecLabel: []lx.DomainDeviceSecLabel{
					lx.DomainDeviceSecLabel{
						Model: "dac",
						Label: fmt.Sprintf("+%d:+%d", file.OwnerUID,
							file.OwnerGID),
					},
				},
			},
		}
	}

	var targetPort uint = 0
	return lx.DomainConsole{
		Source: source,
		Target: &lx.DomainConsoleTarget{
			Port: &targetPort,
		},
	}
}

func libvirtDisk(pool, volume, bus, dev string) lx.DomainDisk {
	return lx.DomainDisk{
		Device: "disk",
		Driver: &lx.DomainDiskDriver{
			Name:    "qemu",
			Type:    "qcow2",
			Discard: "unmap",
		},
		Source: &lx.DomainDiskSource{
			Volume: &lx.DomainDiskSourceVolume{
				Pool:   pool,
				Volume: volume,
			},
		},
		Target: &lx.DomainDiskTarget{
			Dev: dev,
			Bus: bus,
		},
	}
}

func libvirtCDROM(pool, volume, bus, dev string) lx.DomainDisk {
	return lx.DomainDisk{
		Device: "cdrom",
		Driver: &lx.DomainDiskDriver{
			Name: "qemu",
			Type: "raw",
		},
		Source: &lx.DomainDiskSource{
			Volume: &lx.DomainDiskSourceVolume{
				Pool:   pool,
				Volume: volume,
			},
		},
		Target: &lx.DomainDiskTarget{
			Dev: dev,
			Bus: bus,
		},
	}
}
