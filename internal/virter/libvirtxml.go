package virter

import (
	"fmt"

	"github.com/LINBIT/virter/pkg/driveletter"
	libvirt "github.com/digitalocean/go-libvirt"
	lx "github.com/libvirt/libvirt-go-xml"
	log "github.com/sirupsen/logrus"
)

type VMDiskDevice string

const (
	VMDiskDeviceDisk  = "disk"
	VMDiskDeviceCDROM = "cdrom"
)

var busToDevPrefix = map[string]string{
	"ide":    "hd",
	"scsi":   "sd",
	"virtio": "vd",
}

var approvedDiskFormats = map[string]bool{
	// only support raw and qcow2 for now...
	"qcow2": true,
	"raw":   true,
}

type VMDisk struct {
	device     VMDiskDevice
	poolName   string
	volumeName string
	bus        string
	format     string
}

func vmDisksToLibvirtDisks(vmDisks []VMDisk) ([]lx.DomainDisk, error) {
	devCounts := map[string]*driveletter.DriveLetter{}

	var result []lx.DomainDisk
	for _, d := range vmDisks {
		driver := map[VMDiskDevice]lx.DomainDiskDriver{
			VMDiskDeviceDisk: lx.DomainDiskDriver{
				Name:    "qemu",
				Discard: "unmap",
				Type:    d.format,
			},
			VMDiskDeviceCDROM: lx.DomainDiskDriver{
				Name: "qemu",
				Type: d.format,
			},
		}[d.device]

		count, ok := devCounts[d.bus]
		if !ok {
			count = driveletter.New()
		}

		devPrefix, ok := busToDevPrefix[d.bus]
		if !ok {
			return nil, fmt.Errorf("on disk '%s': invalid bus type '%s'",
				d.volumeName, d.bus)
		}
		devLetter := count.String()

		count.Inc()
		devCounts[d.bus] = count

		disk := lx.DomainDisk{
			Device: string(d.device),
			Driver: &driver,
			Source: &lx.DomainDiskSource{
				Volume: &lx.DomainDiskSourceVolume{
					Pool:   d.poolName,
					Volume: d.volumeName,
				},
			},
			Target: &lx.DomainDiskTarget{
				Dev: devPrefix + devLetter,
				Bus: d.bus,
			},
		}

		result = append(result, disk)
	}

	return result, nil
}

func (v *Virter) vmXML(poolName string, vm VMConfig, mac string) (string, error) {
	vmDisks := []VMDisk{
		VMDisk{device: VMDiskDeviceDisk, poolName: poolName, volumeName: vm.Name, bus: "virtio", format: "qcow2"},
		VMDisk{device: VMDiskDeviceCDROM, poolName: poolName, volumeName: ciDataVolumeName(vm.Name), bus: "ide", format: "raw"},
	}
	for _, d := range vm.Disks {
		disk := VMDisk{device: VMDiskDeviceDisk, poolName: poolName, volumeName: diskVolumeName(vm.Name, d.GetName()), bus: d.GetBus(), format: d.GetFormat()}
		vmDisks = append(vmDisks, disk)
	}

	log.Debugf("input are these vmdisks: %+v", vmDisks)
	disks, err := vmDisksToLibvirtDisks(vmDisks)
	if err != nil {
		return "", fmt.Errorf("failed to build libvirt disks: %w", err)
	}
	log.Debugf("output are these disks: %+v", disks)

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
			Disks: disks,
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
				libvirtConsole(vm),
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

func (v *Virter) diskVolumeXML(name string, sizeKiB uint64, format string) (string, error) {
	volume := &lx.StorageVolume{
		Name:     name,
		Capacity: &lx.StorageVolumeSize{Value: sizeKiB, Unit: "KiB"},
		Target: &lx.StorageVolumeTarget{
			Format: &lx.StorageVolumeTargetFormat{Type: format},
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

func libvirtConsole(vm VMConfig) lx.DomainConsole {
	var source *lx.DomainChardevSource
	// no dir -> just return a regular PTY console
	if vm.ConsoleDir == nil {
		source = &lx.DomainChardevSource{
			Pty: &lx.DomainChardevSourcePty{},
		}
	} else {
		path := vm.ConsoleLogPath()
		log.Debugf("Logging VM console output to %s", path)

		source = &lx.DomainChardevSource{
			File: &lx.DomainChardevSourceFile{
				Path:   path,
				Append: "off",
				SecLabel: []lx.DomainDeviceSecLabel{
					lx.DomainDeviceSecLabel{
						Model: "dac",
						Label: fmt.Sprintf("+%d:+%d",
							vm.ConsoleDir.OwnerUID,
							vm.ConsoleDir.OwnerGID),
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

func (v *Virter) getDisksOfDomain(domain libvirt.Domain) ([]string, error) {
	xml, err := v.libvirt.DomainGetXMLDesc(domain, 0)
	if err != nil {
		return nil, fmt.Errorf("could not get domain XML: %w", err)
	}

	domcfg := &lx.Domain{}
	err = domcfg.Unmarshal(xml)
	if err != nil {
		return nil, fmt.Errorf("failed to parse domain XML: %w", err)
	}

	var result []string
	if domcfg.Devices == nil {
		log.Debugf("Domain '%s' has no <devices> section, returning no results", domcfg.Name)
		return result, nil
	}

	for i, disk := range domcfg.Devices.Disks {
		if disk.Source == nil || disk.Source.Volume == nil {
			log.Debugf("Skipping disk without valid <source> section (#%d of domain '%s')",
				i, domcfg.Name)
			continue
		}
		result = append(result, disk.Source.Volume.Volume)
	}

	log.Debugf("found these disks for domain '%s': %v", domcfg.Name, result)

	return result, nil
}
