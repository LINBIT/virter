package virter

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"

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

func vmDisksToLibvirtDisks(vmDisks []VMDisk, diskCache string) ([]lx.DomainDisk, error) {
	devCounts := map[string]*driveletter.DriveLetter{}

	var result []lx.DomainDisk
	for _, d := range vmDisks {
		driver := map[VMDiskDevice]lx.DomainDiskDriver{
			VMDiskDeviceDisk: lx.DomainDiskDriver{
				Name:    "qemu",
				Cache:   diskCache,
				Discard: "unmap",
				Type:    d.format,
			},
			VMDiskDeviceCDROM: lx.DomainDiskDriver{
				Name:  "qemu",
				Cache: diskCache,
				Type:  d.format,
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

func (v *Virter) vmXML(vm VMConfig, mac string, meta *VMMeta) (string, error) {
	vmDisks := []VMDisk{
		VMDisk{device: VMDiskDeviceDisk, poolName: v.provisionStoragePool.Name, volumeName: DynamicLayerName(vm.Name), bus: "virtio", format: "qcow2"},
		VMDisk{device: VMDiskDeviceCDROM, poolName: v.provisionStoragePool.Name, volumeName: DynamicLayerName(ciDataVolumeName(vm.Name)), bus: "scsi", format: "raw"},
	}
	for _, d := range vm.Disks {
		pool := d.GetPool()
		if pool == "" {
			pool = v.provisionStoragePool.Name
		}
		vmDisks = append(vmDisks, VMDisk{
			device:     VMDiskDeviceDisk,
			poolName:   pool,
			volumeName: DynamicLayerName(diskVolumeName(vm.Name, d.GetName())),
			bus:        d.GetBus(),
			format:     d.GetFormat(),
		})
	}

	log.Debugf("input are these vmdisks: %+v", vmDisks)
	disks, err := vmDisksToLibvirtDisks(vmDisks, vm.DiskCache)
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

	var vncGraphics *lx.DomainGraphicVNC
	if vm.VNCEnabled {
		vncGraphics = &lx.DomainGraphicVNC{
			Port:   vm.VNCPort,
			Listen: vm.VNCIPv4BindAddress,
		}
	}

	domain := &lx.Domain{
		Type: vm.CpuArch.DomainType(),
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
			Value:     vm.VCPUs,
		},
		OS: vm.CpuArch.OSDomain(),
		Features: &lx.DomainFeatureList{
			ACPI: &lx.DomainFeature{},
			APIC: &lx.DomainFeatureAPIC{},
		},
		CPU: vm.CpuArch.CPU(),
		Clock: &lx.DomainClock{
			Offset: "utc",
			Timer: []lx.DomainTimer{
				{Name: "rtc", TickPolicy: "catchup"},
				{Name: "pit", TickPolicy: "delay"},
				{Name: "hpet", Present: "no"},
			},
		},
		OnPoweroff: "destroy",
		OnReboot:   "restart",
		OnCrash:    "destroy",
		PM:         vm.CpuArch.PM(),
		Devices: &lx.DomainDeviceList{
			Disks: disks,
			Controllers: []lx.DomainController{
				{
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
			Graphics: []lx.DomainGraphic{
				{VNC: vncGraphics},
			},
			// For some reason, debian stretch doesn't boot without a video card. The virtio model seems to stable
			// enough, even for multi-arch scenarios.
			Videos: []lx.DomainVideo{
				{Model: lx.DomainVideoModel{Type: "virtio"}},
			},
			MemBalloon: &lx.DomainMemBalloon{
				Model: "virtio",
				Alias: &lx.DomainAlias{
					Name: "ballon0",
				},
			},
			// Simulate hardware RNG. This should help the guest OS to reach the required amount of entropy early in
			// the boot process.
			RNGs: []lx.DomainRNG{
				{Model: "virtio", Backend: &lx.DomainRNGBackend{Random: &lx.DomainRNGBackendRandom{Device: "/dev/urandom"}}},
			},
		},
		QEMUCommandline: qemuCommandline,
	}

	err = addMounts(domain, vm.Mounts...)
	if err != nil {
		return "", err
	}

	if vm.SecureBoot {
		if domain.OS.Loader == nil {
			domain.OS.Loader = &lx.DomainLoader{}
		}

		domain.OS.Loader.Secure = "yes"
		domain.OS.Firmware = "efi"
		domain.Features.SMM = &lx.DomainFeatureSMM{}
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

func (v *Virter) getDisksOfDomain(domain libvirt.Domain) ([]VMDisk, error) {
	xml, err := v.libvirt.DomainGetXMLDesc(domain, 0)
	if err != nil {
		return nil, fmt.Errorf("could not get domain XML: %w", err)
	}

	domcfg := &lx.Domain{}
	err = domcfg.Unmarshal(xml)
	if err != nil {
		return nil, fmt.Errorf("failed to parse domain XML: %w", err)
	}

	var result []VMDisk
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
		if disk.Target == nil {
			log.Debugf("Skipping disk without valid <target> section (#%d of domain '%s')",
				i, domcfg.Name)
			continue
		}
		if disk.Driver == nil || !approvedDiskFormats[disk.Driver.Type] {
			log.Debugf("Skipping disk without valid <driver> section (#%d of domain '%s')",
				i, domcfg.Name)
			continue
		}

		result = append(result, VMDisk{
			device:     VMDiskDevice(disk.Device),
			poolName:   disk.Source.Volume.Pool,
			volumeName: disk.Source.Volume.Volume,
			bus:        disk.Target.Bus,
			format:     disk.Driver.Type,
		})
	}

	log.Debugf("found these disks for domain '%s': %v", domcfg.Name, result)

	return result, nil
}

// addMounts adds the virtiofs stanzas required for the specified mounts.
func addMounts(domain *lx.Domain, mounts ...Mount) error {
	if len(mounts) == 0 {
		// Some libvirt versions to not support virtiofs, bail out early so virter is still usable in those cases
		return nil
	}

	fsShares := make([]lx.DomainFilesystem, len(mounts))
	for i, share := range mounts {
		abs, err := filepath.Abs(share.GetHostPath())
		if err != nil {
			return fmt.Errorf("invalid host path: %w", err)
		}

		err = os.MkdirAll(abs, 0755)
		if err != nil {
			return fmt.Errorf("invalid host path: %w", err)
		}

		fsShares[i].Driver = &lx.DomainFilesystemDriver{Type: "virtiofs"}
		fsShares[i].Source = &lx.DomainFilesystemSource{Mount: &lx.DomainFilesystemSourceMount{Dir: abs}}
		fsShares[i].Target = &lx.DomainFilesystemTarget{Dir: share.GetVMPath()}
	}

	domain.Devices.Filesystems = fsShares

	// Required for virtiofs to work: https://libvirt.org/kbase/virtiofs.html#sharing-a-host-directory-with-a-guest
	domain.MemoryBacking = &lx.DomainMemoryBacking{
		MemorySource: &lx.DomainMemorySource{Type: "memfd"},
		MemoryAccess: &lx.DomainMemoryAccess{Mode: "shared"},
	}

	return nil
}
