package virter

import (
	"encoding/xml"
	"fmt"

	"github.com/digitalocean/go-libvirt"
	"libvirt.org/go/libvirtxml"
)

func (v *Virter) NetworkGet(netname string) (*libvirtxml.Network, error) {
	net, err := v.libvirt.NetworkLookupByName(netname)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup network '%s': %w", netname, err)
	}

	desc, err := v.libvirt.NetworkGetXMLDesc(net, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch network xml '%s': %w", netname, err)
	}

	var result libvirtxml.Network
	err = xml.Unmarshal([]byte(desc), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to convert network '%s' xml to go object: %w", netname, err)
	}

	return &result, err
}

func (v *Virter) NetworkList() ([]libvirtxml.Network, error) {
	networks, _, err := v.libvirt.ConnectListAllNetworks(-1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list available networks: %w", err)
	}

	xmllist := make([]libvirtxml.Network, len(networks))
	for i, lnet := range networks {
		desc, err := v.libvirt.NetworkGetXMLDesc(lnet, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch network xml '%s': %w", lnet.Name, err)
		}

		err = xml.Unmarshal([]byte(desc), &xmllist[i])
		if err != nil {
			return nil, fmt.Errorf("failed to convert network '%s' xml to go object: %w", lnet.Name, err)
		}
	}

	return xmllist, nil
}

func (v *Virter) NetworkAdd(desc libvirtxml.Network) error {
	xmlstring, err := xml.Marshal(desc)
	if err != nil {
		return fmt.Errorf("failed to marshal network '%s' xml to string: %w", desc.Name, err)
	}

	lnet, err := v.libvirt.NetworkDefineXML(string(xmlstring))
	if err != nil {
		return fmt.Errorf("could not define network '%s': %w", desc.Name, err)
	}

	err = v.libvirt.NetworkSetAutostart(lnet, 1)
	if err != nil {
		return fmt.Errorf("failed to set network '%s' to autostart: %w", desc.Name, err)
	}

	err = v.libvirt.NetworkCreate(lnet)
	if err != nil {
		return fmt.Errorf("failed to start network '%s': %w", desc.Name, err)
	}

	return nil
}

func (v *Virter) NetworkRemove(netname string) error {
	lnet, err := v.libvirt.NetworkLookupByName(netname)
	if err != nil {
		if hasErrorCode(err, libvirt.ErrNoNetwork) {
			return nil
		}
		return fmt.Errorf("failed to lookup network '%s': %w", netname, err)
	}

	err = v.libvirt.NetworkDestroy(lnet)
	if err != nil && !hasErrorCode(err, libvirt.ErrOperationInvalid) {
		// We intentionally ignore ErrOperationInvalid, as that indicates an inactive network, i.e. this step
		// is already done.
		return fmt.Errorf("failed to stop network '%s': %w", netname, err)
	}

	err = v.libvirt.NetworkUndefine(lnet)
	if err != nil {
		return fmt.Errorf("failed to undefine network '%s': %w", netname, err)
	}

	return nil
}

type VMNic struct {
	VMName     string
	MAC        string
	IP         string
	HostName   string
	HostDevice string
}

func (v *Virter) NetworkListAttached(netname string) ([]VMNic, error) {
	lnet, err := v.libvirt.NetworkLookupByName(netname)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup network '%s': %w", netname, err)
	}

	leases, _, err := v.libvirt.NetworkGetDhcpLeases(lnet, nil, 1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch leases: %w", err)
	}

	domains, _, err := v.libvirt.ConnectListAllDomains(-1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}

	macToLease := make(map[string]libvirt.NetworkDhcpLease)
	for _, lease := range leases {
		for _, leaseMac := range lease.Mac {
			macToLease[leaseMac] = lease
		}
	}

	var vmnics []VMNic
	for _, d := range domains {
		nics, err := v.getNICs(d)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch interfaces for domain '%s': %w", d.Name, err)
		}

		for _, nic := range nics {
			if nic.Network != netname {
				continue
			}

			var hostname, ip string
			lease, ok := macToLease[nic.MAC]
			if ok {
				ip = lease.Ipaddr
				if len(lease.Hostname) > 0 {
					hostname = lease.Hostname[0]
				}
			}

			vmnics = append(vmnics, VMNic{
				VMName:     d.Name,
				MAC:        nic.MAC,
				HostDevice: nic.HostDevice,
				IP:         ip,
				HostName:   hostname,
			})
		}
	}

	return vmnics, nil
}

// DrvFeatureNetworkUpdateHasCorrectOrder determines wheter the argument order for NetworkUpdate is fixed
// From https://github.com/libvirt/libvirt/blob/cac9608c172e9439d9cba350579af59b649b77e7/src/libvirt_internal.h#L133
const DrvFeatureNetworkUpdateHasCorrectOrder = 16

// patchedNetworkUpdate calls libvirt's NetworkUpdate(), with the right order of arguments.
func (v *Virter) patchedNetworkUpdate(network libvirt.Network, command libvirt.NetworkUpdateCommand, section libvirt.NetworkUpdateSection, xml string) error {
	hasCorrectOrder, err := v.libvirt.ConnectSupportsFeature(DrvFeatureNetworkUpdateHasCorrectOrder)
	if err != nil {
		return fmt.Errorf("failed to check for fixed network update argument order: %w", err)
	}

	argCommand := uint32(command)
	argSection := uint32(section)

	if hasCorrectOrder != 1 {
		argCommand = uint32(section)
		argSection = uint32(command)
	}

	err = v.libvirt.NetworkUpdate(
		network,
		argCommand,
		argSection,
		-1,
		xml,
		libvirt.NetworkUpdateAffectLive|libvirt.NetworkUpdateAffectConfig)
	if err != nil {
		return err
	}

	return nil
}
