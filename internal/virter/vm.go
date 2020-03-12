package virter

import (
	"bytes"
	"fmt"
	"log"
	"net"

	"github.com/digitalocean/go-libvirt"
)

// ISOGenerator generates ISO images from file data
type ISOGenerator interface {
	Generate(files map[string][]byte) ([]byte, error)
}

// VMRun starts a VM.
func (v *Virter) VMRun(g ISOGenerator, imageName string, vmName string, vmID uint, sshPublicKey string) error {
	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	log.Print("Create boot volume")
	err = v.createVMVolume(sp, imageName, vmName)
	if err != nil {
		return err
	}

	log.Print("Create cloud-init volume")
	err = v.createCIData(sp, g, vmName, sshPublicKey)
	if err != nil {
		return err
	}

	log.Print("Create scratch volume")
	err = v.createScratchVolume(sp, vmName)
	if err != nil {
		return err
	}

	err = v.createVM(sp, vmName, vmID)
	if err != nil {
		return err
	}

	return nil
}

func (v *Virter) createCIData(sp libvirt.StoragePool, g ISOGenerator, vmName string, sshPublicKey string) error {
	metaData, err := v.metaData(vmName)
	if err != nil {
		return err
	}

	userData, err := v.userData(vmName, sshPublicKey)
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

func (v *Virter) userData(vmName string, sshPublicKey string) (string, error) {
	templateData := map[string]interface{}{
		"VMName":       vmName,
		"SSHPublicKey": sshPublicKey,
	}

	return v.renderTemplate(templateUserData, templateData)
}

func (v *Virter) ciDataVolumeXML(name string) (string, error) {
	templateData := map[string]interface{}{
		"VolumeName": name,
	}

	return v.renderTemplate(templateCIData, templateData)
}

func (v *Virter) createVMVolume(sp libvirt.StoragePool, imageName string, vmName string) error {
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

func (v *Virter) createScratchVolume(sp libvirt.StoragePool, vmName string) error {
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

func (v *Virter) createVM(sp libvirt.StoragePool, vmName string, vmID uint) error {
	mac := qemuMAC(vmID)

	xml, err := v.vmXML(sp.Name, vmName, mac)
	if err != nil {
		return err
	}

	log.Print("Define VM")
	err = v.createScratchVolume(sp, vmName)
	d, err := v.libvirt.DomainDefineXML(xml)
	if err != nil {
		return fmt.Errorf("could not define domain: %w", err)
	}

	// Add DHCP entry after defining the VM to ensure that it can be
	// removed when removing the VM, but before starting it to ensure that
	// it gets the correct IP address
	err = v.addDHCPEntry(mac, vmID)
	if err != nil {
		return err
	}

	log.Print("Start VM")
	err = v.libvirt.DomainCreate(d)
	if err != nil {
		return fmt.Errorf("could create create (start) domain: %w", err)
	}

	return nil
}

// addDHCPEntry adds a DHCP mapping from a MAC address to an IP generated from
// the id. The same MAC address should always be paired with a given IP so that
// DHCP entries do not need to be released between removing a VM and creating
// another with the same ID.
func (v *Virter) addDHCPEntry(mac string, id uint) error {
	network, err := v.libvirt.NetworkLookupByName(v.networkName)
	if err != nil {
		return fmt.Errorf("could not get network: %w", err)
	}

	networkDescription, err := getNetworkDescription(v.libvirt, network)
	if err != nil {
		return err
	}
	if len(networkDescription.IPs) < 1 {
		return fmt.Errorf("no IPs in network")
	}

	ipDescription := networkDescription.IPs[0]
	if ipDescription.Address == "" {
		return fmt.Errorf("could not find address in network XML")
	}
	if ipDescription.Netmask == "" {
		return fmt.Errorf("could not find netmask in network XML")
	}

	networkIP := net.ParseIP(ipDescription.Address)
	if networkIP == nil {
		return fmt.Errorf("could not parse network IP address")
	}

	networkMaskIP := net.ParseIP(ipDescription.Netmask)
	if networkMaskIP == nil {
		return fmt.Errorf("could not parse network mask IP address")
	}

	networkMaskIPv4 := networkMaskIP.To4()
	if networkMaskIPv4 == nil {
		return fmt.Errorf("network mask is not IPv4 address")
	}

	networkMask := net.IPMask(networkMaskIPv4)
	networkBaseIP := networkIP.Mask(networkMask)
	ip := addToIP(networkBaseIP, id)

	networkIPNet := net.IPNet{IP: networkIP, Mask: networkMask}
	if !networkIPNet.Contains(ip) {
		return fmt.Errorf("computed IP %v is not in network", ip)
	}

	log.Printf("Add DHCP entry from %v to %v", mac, ip)
	err = v.libvirt.NetworkUpdate(
		network,
		// the following 2 arguments are swapped; see
		// https://github.com/digitalocean/go-libvirt/issues/87
		uint32(libvirt.NetworkSectionIPDhcpHost),
		uint32(libvirt.NetworkUpdateCommandAddLast),
		-1,
		fmt.Sprintf("<host mac='%s' ip='%v'/>", mac, ip),
		libvirt.NetworkUpdateAffectLive|libvirt.NetworkUpdateAffectConfig)
	if err != nil {
		return fmt.Errorf("could not add DHCP entry: %w", err)
	}

	return nil
}

func addToIP(ip net.IP, addend uint) net.IP {
	i := ip.To4()
	v := uint(i[0])<<24 + uint(i[1])<<16 + uint(i[2])<<8 + uint(i[3])
	v += addend
	v0 := byte((v >> 24) & 0xFF)
	v1 := byte((v >> 16) & 0xFF)
	v2 := byte((v >> 8) & 0xFF)
	v3 := byte(v & 0xFF)
	return net.IPv4(v0, v1, v2, v3)
}

func qemuMAC(id uint) string {
	id0 := byte((id >> 16) & 0xFF)
	id1 := byte((id >> 8) & 0xFF)
	id2 := byte(id & 0xFF)
	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", id0, id1, id2)
}

func (v *Virter) vmXML(poolName string, vmName string, mac string) (string, error) {
	templateData := map[string]interface{}{
		"PoolName": poolName,
		"VMName":   vmName,
		"MAC":      mac,
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

		if active != 0 {
			log.Print("Stop VM")
			err = v.libvirt.DomainDestroy(domain)
			if err != nil {
				return fmt.Errorf("could not destroy domain: %w", err)
			}
		}

		err = v.rmDHCPEntry(domain)
		if err != nil {
			return err
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

func (v *Virter) rmDHCPEntry(domain libvirt.Domain) error {
	domainDescription, err := getDomainDescription(v.libvirt, domain)
	if err != nil {
		return err
	}

	devicesDescription := domainDescription.Devices
	if devicesDescription == nil {
		return fmt.Errorf("no devices in domain")
	}
	if len(devicesDescription.Interfaces) < 1 {
		return fmt.Errorf("no interface devices in domain")
	}

	interfaceDescription := devicesDescription.Interfaces[0]

	macDescription := interfaceDescription.MAC
	if macDescription == nil {
		return fmt.Errorf("no MAC in domain interface device")
	}

	mac := macDescription.Address

	network, err := v.libvirt.NetworkLookupByName(v.networkName)
	if err != nil {
		return fmt.Errorf("could not get network: %w", err)
	}

	networkDescription, err := getNetworkDescription(v.libvirt, network)
	if err != nil {
		return err
	}
	if len(networkDescription.IPs) < 1 {
		return fmt.Errorf("no IPs in network")
	}

	ipDescription := networkDescription.IPs[0]

	dhcpDescription := ipDescription.DHCP
	if dhcpDescription == nil {
		return fmt.Errorf("no DHCP in network")
	}

	for _, host := range dhcpDescription.Hosts {
		if host.MAC == mac {
			log.Printf("Remove DHCP entry from %v to %v", mac, host.IP)
			err = v.libvirt.NetworkUpdate(
				network,
				// the following 2 arguments are swapped; see
				// https://github.com/digitalocean/go-libvirt/issues/87
				uint32(libvirt.NetworkSectionIPDhcpHost),
				uint32(libvirt.NetworkUpdateCommandDelete),
				-1,
				fmt.Sprintf("<host mac='%s' ip='%v'/>", mac, host.IP),
				libvirt.NetworkUpdateAffectLive|libvirt.NetworkUpdateAffectConfig)
			if err != nil {
				return fmt.Errorf("could not remove DHCP entry: %w", err)
			}
		}
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

// VMCommit commits a VM to an image.
func (v *Virter) VMCommit(vmName string) error {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if err != nil {
		return fmt.Errorf("could not get domain: %w", err)
	}

	active, err := v.libvirt.DomainIsActive(domain)
	if err != nil {
		return fmt.Errorf("could not check if domain is active: %w", err)
	}

	if active != 0 {
		return fmt.Errorf("cannot commit a running VM")
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

const templateMetaData = "meta-data"
const templateUserData = "user-data"
const templateCIData = "volume-cidata.xml"
const templateVMVolume = "volume-vm.xml"
const templateScratchVolume = "volume-scratch.xml"
const templateVM = "vm.xml"
