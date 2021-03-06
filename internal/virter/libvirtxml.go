package virter

import (
	"encoding/xml"
	"fmt"

	libvirt "github.com/digitalocean/go-libvirt"
	lx "github.com/libvirt/libvirt-go-xml"
	log "github.com/sirupsen/logrus"

	"github.com/LINBIT/virter/pkg/driveletter"
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

func (v *Virter) vmXML(poolName string, vm VMConfig, mac string, meta *VMMeta) (string, error) {
	vmDisks := []VMDisk{
		VMDisk{device: VMDiskDeviceDisk, poolName: poolName, volumeName: DynamicLayerName(vm.Name), bus: "virtio", format: "qcow2"},
		VMDisk{device: VMDiskDeviceCDROM, poolName: poolName, volumeName: DynamicLayerName(ciDataVolumeName(vm.Name)), bus: "ide", format: "raw"},
	}
	for _, d := range vm.Disks {
		disk := VMDisk{device: VMDiskDeviceDisk, poolName: poolName, volumeName: DynamicLayerName(diskVolumeName(vm.Name, d.GetName())), bus: d.GetBus(), format: d.GetFormat()}
		vmDisks = append(vmDisks, disk)
	}

	log.Debugf("input are these vmdisks: %+v", vmDisks)
	disks, err := vmDisksToLibvirtDisks(vmDisks)
	if err != nil {
		return "", fmt.Errorf("failed to build libvirt disks: %w", err)
	}
	log.Debugf("output are these disks: %+v", disks)

	var qemuCommandline *lx.DomainQEMUCommandline
	if vm.GDBPort != 0 {
		qemuCommandline = &lx.DomainQEMUCommandline{
			Args: []lx.DomainQEMUCommandlineArg{
				{Value: "-gdb"},
				{Value: fmt.Sprintf("tcp::%d", vm.GDBPort)},
			},
		}
	}

	log.Debugf("adding extra NIC to vm for: %v", vm.ExtraNics)
	extraNICs, err := vmNICtoLibvirtInterfaces(vm.ExtraNics)
	if err != nil {
		return "", err
	}
	log.Debugf("output are these interfaces: %v", extraNICs)

	metaXml, err := xml.Marshal(metaWrapper{VMMeta: meta})
	if err != nil {
		return "", fmt.Errorf("failed to create metadata xml: %w", err)
	}

	domain := &lx.Domain{
		Type: "kvm",
		Name: vm.Name,
		Metadata: &lx.DomainMetadata{
			XML: string(metaXml),
		},
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
			Interfaces: append(
				[]lx.DomainInterface{{
					MAC: &lx.DomainInterfaceMAC{
						Address: mac,
					},
					Source: &lx.DomainInterfaceSource{
						Network: &lx.DomainInterfaceSourceNetwork{
							Network: v.provisionNetwork.Name,
						},
					},
					Model: &lx.DomainInterfaceModel{
						Type: "virtio",
					},
				}},
				extraNICs...,
			),
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
		QEMUCommandline: qemuCommandline,
	}
	return domain.Marshal()
}

func vmNICtoLibvirtInterfaces(nics []NIC) ([]lx.DomainInterface, error) {
	lnics := make([]lx.DomainInterface, len(nics))
	for i, nic := range nics {
		var source *lx.DomainInterfaceSource
		switch nic.GetType() {
		case NICTypeBridge:
			source = &lx.DomainInterfaceSource{Bridge: &lx.DomainInterfaceSourceBridge{Bridge: nic.GetSource()}}
		case NICTypeNetwork:
			source = &lx.DomainInterfaceSource{Network: &lx.DomainInterfaceSourceNetwork{Network: nic.GetSource()}}
		default:
			return nil, fmt.Errorf("unsupported interface type: %s", nic.GetType())
		}

		var mac *lx.DomainInterfaceMAC
		if nic.GetMAC() != "" {
			mac = &lx.DomainInterfaceMAC{Address: nic.GetMAC()}
		}

		lnics[i] = lx.DomainInterface{
			Source: source,
			MAC:    mac,
			Model:  &lx.DomainInterfaceModel{Type: nic.GetModel()},
		}
	}

	return lnics, nil
}

func (v *Virter) ciDataVolumeXML(name string) (string, error) {
	return v.diskVolumeXML(name, 0, "B", "raw")
}

func (v *Virter) vmVolumeXML(name, backingPath string, sizeB uint64) (string, error) {
	volume := v.diskVolume(name, sizeB, "B", "qcow2")
	volume.BackingStore = &lx.StorageVolumeBackingStore{
		Path:   backingPath,
		Format: &lx.StorageVolumeTargetFormat{Type: "qcow2"},
	}
	return volume.Marshal()
}

func (v *Virter) diskVolume(name string, size uint64, unit, format string) *lx.StorageVolume {
	return &lx.StorageVolume{
		Name:     name,
		Capacity: &lx.StorageVolumeSize{Value: size, Unit: unit},
		Target: &lx.StorageVolumeTarget{
			Format: &lx.StorageVolumeTargetFormat{Type: format},
		},
	}
}

func (v *Virter) diskVolumeXML(name string, size uint64, unit, format string) (string, error) {
	return v.diskVolume(name, size, unit, format).Marshal()
}

func (v *Virter) imageVolumeXML(name string) (string, error) {
	return v.diskVolumeXML(name, 0, "B", "qcow2")
}

func libvirtConsole(vm VMConfig) lx.DomainConsole {
	var source *lx.DomainChardevSource
	// no dir -> just return a regular PTY console
	if vm.ConsolePath == "" {
		source = &lx.DomainChardevSource{
			Pty: &lx.DomainChardevSourcePty{},
		}
	} else {
		log.Debugf("Logging VM console output to %s", vm.ConsolePath)

		source = &lx.DomainChardevSource{
			File: &lx.DomainChardevSourceFile{
				Path:   vm.ConsolePath,
				Append: "on",
				SecLabel: []lx.DomainDeviceSecLabel{
					lx.DomainDeviceSecLabel{
						Model:   "dac",
						Relabel: "no",
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

type metaWrapper struct {
	XMLName xml.Name `xml:"https://github.com/LINBIT/virter meta"`
	*VMMeta
}

func (v *Virter) getMetaForVM(vmName string) (*VMMeta, error) {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if err != nil {
		return nil, fmt.Errorf("could not find domain: '%s': %w", vmName, err)
	}

	xmldesc, err := v.libvirt.DomainGetXMLDesc(domain, 0)
	if err != nil {
		return nil, fmt.Errorf("could not get domain xml '%s': %w", vmName, err)
	}

	desc := lx.Domain{}
	err = xml.Unmarshal([]byte(xmldesc), &desc)
	if err != nil {
		return nil, fmt.Errorf("could not decode domain xml '%s': %w", vmName, err)
	}

	meta := metaWrapper{}
	err = xml.Unmarshal([]byte(desc.Metadata.XML), &meta)
	if err != nil {
		return nil, fmt.Errorf("could not decode meta xml: '%s': %w", vmName, err)
	}

	return meta.VMMeta, nil
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
