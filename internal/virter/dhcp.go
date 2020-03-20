package virter

import (
	"fmt"
	"log"
	"net"

	"github.com/digitalocean/go-libvirt"
)

// addDHCPEntry adds a DHCP mapping from a MAC address to an IP generated from
// the id. The same MAC address should always be paired with a given IP so that
// DHCP entries do not need to be released between removing a VM and creating
// another with the same ID.
func (v *Virter) addDHCPEntry(mac string, id uint) (net.IP, error) {
	network, err := v.libvirt.NetworkLookupByName(v.networkName)
	if err != nil {
		return nil, fmt.Errorf("could not get network: %w", err)
	}

	ipNet, err := v.getIPNet(network)
	if err != nil {
		return nil, err
	}

	networkBaseIP := ipNet.IP.Mask(ipNet.Mask)
	ip := addToIP(networkBaseIP, id)

	if !ipNet.Contains(ip) {
		return nil, fmt.Errorf("computed IP %v is not in network", ip)
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
		return nil, fmt.Errorf("could not add DHCP entry: %w", err)
	}

	return ip, nil
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

func (v *Virter) getIPNet(network libvirt.Network) (net.IPNet, error) {
	ipNet := net.IPNet{}

	networkDescription, err := getNetworkDescription(v.libvirt, network)
	if err != nil {
		return ipNet, err
	}
	if len(networkDescription.IPs) < 1 {
		return ipNet, fmt.Errorf("no IPs in network")
	}

	ipDescription := networkDescription.IPs[0]
	if ipDescription.Address == "" {
		return ipNet, fmt.Errorf("could not find address in network XML")
	}
	if ipDescription.Netmask == "" {
		return ipNet, fmt.Errorf("could not find netmask in network XML")
	}

	ipNet.IP = net.ParseIP(ipDescription.Address)
	if ipNet.IP == nil {
		return ipNet, fmt.Errorf("could not parse network IP address")
	}

	networkMaskIP := net.ParseIP(ipDescription.Netmask)
	if networkMaskIP == nil {
		return ipNet, fmt.Errorf("could not parse network mask IP address")
	}

	networkMaskIPv4 := networkMaskIP.To4()
	if networkMaskIPv4 == nil {
		return ipNet, fmt.Errorf("network mask is not IPv4 address")
	}

	ipNet.Mask = net.IPMask(networkMaskIPv4)

	return ipNet, nil
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
