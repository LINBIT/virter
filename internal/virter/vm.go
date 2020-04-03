package virter

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	libvirt "github.com/digitalocean/go-libvirt"
)

// VMRun starts a VM.
func (v *Virter) VMRun(g ISOGenerator, waiter PortWaiter, vmConfig VMConfig, waitSSH bool) error {
	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	log.Print("Create boot volume")
	err = v.createVMVolume(sp, vmConfig)
	if err != nil {
		return err
	}

	log.Print("Create cloud-init volume")
	err = v.createCIData(sp, g, vmConfig)
	if err != nil {
		return err
	}

	log.Print("Create scratch volume")
	err = v.createScratchVolume(sp, vmConfig)
	if err != nil {
		return err
	}

	ip, err := v.createVM(sp, vmConfig)
	if err != nil {
		return err
	}

	if waitSSH {
		log.Print("Wait for SSH port to open")
		err = waiter.WaitPort(ip, "ssh")
		if err != nil {
			return fmt.Errorf("unable to connect to SSH port: %w", err)
		}
		log.Print("Successfully connected to SSH port")
	}

	return nil
}

func (v *Virter) createCIData(sp libvirt.StoragePool, g ISOGenerator, vmConfig VMConfig) error {
	vmName := vmConfig.VMName
	sshPublicKeys := vmConfig.SSHPublicKeys

	metaData, err := v.metaData(vmName)
	if err != nil {
		return err
	}

	userData, err := v.userData(vmName, sshPublicKeys)
	if err != nil {
		return err
	}

	files := map[string][]byte{
		"meta-data": []byte(metaData),
		"user-data": []byte(userData),
	}

	ciData, err := g.Generate(files)
	if err != nil {
		return fmt.Errorf("failed to generate ISO: %w", err)
	}

	xml, err := v.ciDataVolumeXML(ciDataVolumeName(vmName))
	if err != nil {
		return err
	}

	sv, err := v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create cloud-init volume: %w", err)
	}

	err = v.libvirt.StorageVolUpload(sv, bytes.NewReader(ciData), 0, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to transfer cloud-init data to libvirt: %w", err)
	}

	return nil
}

func ciDataVolumeName(vmName string) string {
	return vmName + "-cidata"
}

func (v *Virter) metaData(vmName string) (string, error) {
	templateData := map[string]interface{}{
		"VMName": vmName,
	}

	return v.renderTemplate(templateMetaData, templateData)
}

func (v *Virter) userData(vmName string, sshPublicKeys []string) (string, error) {
	templateData := map[string]interface{}{
		"VMName":        vmName,
		"SSHPublicKeys": sshPublicKeys,
	}

	return v.renderTemplate(templateUserData, templateData)
}

func (v *Virter) ciDataVolumeXML(name string) (string, error) {
	templateData := map[string]interface{}{
		"VolumeName": name,
	}

	return v.renderTemplate(templateCIData, templateData)
}

func (v *Virter) createVMVolume(sp libvirt.StoragePool, vmConfig VMConfig) error {
	imageName := vmConfig.ImageName
	vmName := vmConfig.VMName

	backingVolume, err := v.libvirt.StorageVolLookupByName(sp, imageName)
	if err != nil {
		return fmt.Errorf("could not get backing image volume: %w", err)
	}

	backingPath, err := v.libvirt.StorageVolGetPath(backingVolume)
	if err != nil {
		return fmt.Errorf("could not get backing image path: %w", err)
	}

	xml, err := v.vmVolumeXML(vmName, backingPath)
	if err != nil {
		return err
	}

	_, err = v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create VM boot volume: %w", err)
	}

	return nil
}

func (v *Virter) vmVolumeXML(name string, backingPath string) (string, error) {
	templateData := map[string]interface{}{
		"VolumeName":  name,
		"BackingPath": backingPath,
	}

	return v.renderTemplate(templateVMVolume, templateData)
}

func (v *Virter) createScratchVolume(sp libvirt.StoragePool, vmConfig VMConfig) error {
	vmName := vmConfig.VMName

	xml, err := v.scratchVolumeXML(scratchVolumeName(vmName))
	if err != nil {
		return err
	}

	_, err = v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create scratch volume: %w", err)
	}

	return nil
}

func scratchVolumeName(vmName string) string {
	return vmName + "-scratch"
}

func (v *Virter) scratchVolumeXML(name string) (string, error) {
	templateData := map[string]interface{}{
		"VolumeName": name,
	}

	return v.renderTemplate(templateScratchVolume, templateData)
}

func (v *Virter) createVM(sp libvirt.StoragePool, vmConfig VMConfig) (net.IP, error) {
	vmName := vmConfig.VMName
	vmID := vmConfig.VMID
	memKiB := vmConfig.MemoryKiB
	vcpus := vmConfig.VCPUs
	mac := qemuMAC(vmID)

	xml, err := v.vmXML(sp.Name, vmName, mac, memKiB, vcpus)
	if err != nil {
		return nil, err
	}

	log.Print("Define VM")
	err = v.createScratchVolume(sp, vmConfig)
	d, err := v.libvirt.DomainDefineXML(xml)
	if err != nil {
		return nil, fmt.Errorf("could not define domain: %w", err)
	}

	// Add DHCP entry after defining the VM to ensure that it can be
	// removed when removing the VM, but before starting it to ensure that
	// it gets the correct IP address
	ip, err := v.addDHCPEntry(mac, vmID)
	if err != nil {
		return nil, err
	}

	log.Print("Start VM")
	err = v.libvirt.DomainCreate(d)
	if err != nil {
		return nil, fmt.Errorf("could create create (start) domain: %w", err)
	}

	return ip, nil
}

func (v *Virter) vmXML(poolName string, vmName string, mac string, memKiB uint64, vcpus uint) (string, error) {
	templateData := map[string]interface{}{
		"PoolName":  poolName,
		"VMName":    vmName,
		"MAC":       mac,
		"MemoryKiB": memKiB,
		"VCPUs":     vcpus,
	}

	return v.renderTemplate(templateVM, templateData)
}

// VMRm removes a VM.
func (v *Virter) VMRm(vmName string) error {
	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	err = v.vmRmExceptBoot(sp, vmName)
	if err != nil {
		return err
	}

	err = v.rmVolume(sp, vmName, "boot")
	if err != nil {
		return err
	}

	return nil
}

func (v *Virter) vmRmExceptBoot(sp libvirt.StoragePool, vmName string) error {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if !hasErrorCode(err, errNoDomain) {
		if err != nil {
			return fmt.Errorf("could not get domain: %w", err)
		}

		err = v.rmSnapshots(domain)
		if err != nil {
			return err
		}

		active, err := v.libvirt.DomainIsActive(domain)
		if err != nil {
			return fmt.Errorf("could not check if domain is active: %w", err)
		}

		persistent, err := v.libvirt.DomainIsPersistent(domain)
		if err != nil {
			return fmt.Errorf("could not check if domain is persistent: %w", err)
		}

		err = v.rmDHCPEntry(domain)
		if err != nil {
			return err
		}

		if active != 0 {
			log.Print("Stop VM")
			err = v.libvirt.DomainDestroy(domain)
			if err != nil {
				return fmt.Errorf("could not destroy domain: %w", err)
			}
		}

		if persistent != 0 {
			log.Print("Undefine VM")
			err = v.libvirt.DomainUndefine(domain)
			if err != nil {
				return fmt.Errorf("could not undefine domain: %w", err)
			}
		}
	}

	err = v.rmVolume(sp, scratchVolumeName(vmName), "scratch")
	if err != nil {
		return err
	}

	err = v.rmVolume(sp, ciDataVolumeName(vmName), "cloud-init")
	if err != nil {
		return err
	}

	return nil
}

func (v *Virter) rmSnapshots(domain libvirt.Domain) error {
	snapshots, _, err := v.libvirt.DomainListAllSnapshots(domain, -1, 0)
	if err != nil {
		return fmt.Errorf("could not list snapshots: %w", err)
	}

	for _, snapshot := range snapshots {
		log.Printf("Delete snapshot %v", snapshot.Name)
		err = v.libvirt.DomainSnapshotDelete(snapshot, 0)
		if err != nil {
			return fmt.Errorf("could not delete snapshot: %w", err)
		}
	}

	return nil
}

func (v *Virter) rmVolume(sp libvirt.StoragePool, volumeName string, debugName string) error {
	volume, err := v.libvirt.StorageVolLookupByName(sp, volumeName)
	if !hasErrorCode(err, errNoStorageVol) {
		if err != nil {
			return fmt.Errorf("could not get %v volume: %w", debugName, err)
		}

		log.Printf("Delete %v volume", debugName)
		err = v.libvirt.StorageVolDelete(volume, 0)
		if err != nil {
			return fmt.Errorf("could not delete %v volume: %w", debugName, err)
		}
	}

	return nil
}

// VMCommit commits a VM to an image. If shutdown is true, a goroutine to watch
// for events will be started. This goroutine will only terminate when the
// libvirt connection is closed, so take care of leaking goroutines.
func (v *Virter) VMCommit(afterNotifier AfterNotifier, vmName string, shutdown bool, shutdownTimeout time.Duration) error {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if err != nil {
		return fmt.Errorf("could not get domain: %w", err)
	}

	if shutdown {
		err = v.vmShutdown(afterNotifier, shutdownTimeout, domain)
		if err != nil {
			return err
		}
	} else {
		active, err := v.libvirt.DomainIsActive(domain)
		if err != nil {
			return fmt.Errorf("could not check if domain is active: %w", err)
		}

		if active != 0 {
			return fmt.Errorf("cannot commit a running VM")
		}
	}

	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	err = v.vmRmExceptBoot(sp, vmName)
	if err != nil {
		return err
	}

	return nil
}

func (v *Virter) vmShutdown(afterNotifier AfterNotifier, shutdownTimeout time.Duration, domain libvirt.Domain) error {
	events, err := v.libvirt.LifecycleEvents()
	if err != nil {
		return fmt.Errorf("could not start waiting for events: %w", err)
	}

	// Check whether domain is active after starting event stream
	// to ensure that the shutdown event is not missed.
	active, err := v.libvirt.DomainIsActive(domain)
	if err != nil {
		return fmt.Errorf("could not check if domain is active: %w", err)
	}

	if active != 0 {
		log.Printf("Shut down VM")

		err = v.libvirt.DomainShutdown(domain)
		if err != nil {
			return fmt.Errorf("could not shut down domain: %w", err)
		}

		log.Printf("Wait for VM to stop")
	}

	timeout := afterNotifier.After(shutdownTimeout)

	for active != 0 {
		select {
		case event := <-events:
			if event.Dom.ID == domain.ID && event.Event == int32(libvirt.DomainEventStopped) {
				log.Printf("VM stopped")
				active = 0
			}
		case <-timeout:
			return fmt.Errorf("timed out waiting for domain to stop")
		}
	}

	return nil
}

// VMExec runs a docker container against a VM.
func (v *Virter) VMExec(ctx context.Context, docker DockerClient, vmName string, dockerContainerConfig DockerContainerConfig, sshPrivateKey []byte) error {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if err != nil {
		return fmt.Errorf("could not get domain: %w", err)
	}

	active, err := v.libvirt.DomainIsActive(domain)
	if err != nil {
		return fmt.Errorf("could not check if domain is active: %w", err)
	}

	if active == 0 {
		return fmt.Errorf("cannot exec against VM that is not running")
	}

	mac, err := v.getMAC(domain)
	if err != nil {
		return err
	}

	network, err := v.libvirt.NetworkLookupByName(v.networkName)
	if err != nil {
		return fmt.Errorf("could not get network: %w", err)
	}

	ips, err := v.findIPs(network, mac)
	if err != nil {
		return err
	}
	if len(ips) < 1 {
		return fmt.Errorf("no IP found for domain")
	}

	ip := ips[0]

	err = dockerRun(ctx, docker, dockerContainerConfig, vmName, ip, sshPrivateKey)
	if err != nil {
		return err
	}

	return nil
}

const templateMetaData = "meta-data"
const templateUserData = "user-data"
const templateCIData = "volume-cidata.xml"
const templateVMVolume = "volume-vm.xml"
const templateScratchVolume = "volume-scratch.xml"
const templateVM = "vm.xml"
