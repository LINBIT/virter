package virter

import (
	"fmt"
	"net"
	"os/exec"

	libvirt "github.com/digitalocean/go-libvirt"
	log "github.com/sirupsen/logrus"

	libvirtxml "github.com/libvirt/libvirt-go-xml"
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

// ipToID converts an IP address(with network) to a ID
func ipToID(ipnet net.IPNet, ip net.IP) (uint, error) {
	if !ipnet.Contains(ip) {
		return 0, fmt.Errorf("computed IP %v is not in network", ip)
	}

	si := ipnet.IP.To4()
	sv := uint(si[0])<<24 + uint(si[1])<<16 + uint(si[2])<<8 + uint(si[3])

	i := ip.To4()
	v := uint(i[0])<<24 + uint(i[1])<<16 + uint(i[2])<<8 + uint(i[3])

	return v - sv, nil
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

	for _, ip := range ips {
		log.Printf("Remove DHCP entry from %v to %v", mac, ip)
		err = v.libvirt.NetworkUpdate(
			network,
			// the following 2 arguments are swapped; see
			// https://github.com/digitalocean/go-libvirt/issues/87
			uint32(libvirt.NetworkSectionIPDhcpHost),
			uint32(libvirt.NetworkUpdateCommandDelete),
			-1,
			fmt.Sprintf("<host mac='%s' ip='%v'/>", mac, ip),
			libvirt.NetworkUpdateAffectLive|libvirt.NetworkUpdateAffectConfig)
		if err != nil {
			return fmt.Errorf("could not remove DHCP entry: %w", err)
		}
	}

	err = v.tryReleaseDHCP(mac, ips, network)
	if err != nil {
		log.Debugf("Could not release DHCP lease: %v", err)
	}

	return nil
}

func (v *Virter) getMAC(domain libvirt.Domain) (string, error) {
	domainDescription, err := getDomainDescription(v.libvirt, domain)
	if err != nil {
		return "", err
	}

	devicesDescription := domainDescription.Devices
	if devicesDescription == nil {
		return "", fmt.Errorf("no devices in domain")
	}
	if len(devicesDescription.Interfaces) < 1 {
		return "", fmt.Errorf("no interface devices in domain")
	}

	interfaceDescription := devicesDescription.Interfaces[0]

	macDescription := interfaceDescription.MAC
	if macDescription == nil {
		return "", fmt.Errorf("no MAC in domain interface device")
	}

	return macDescription.Address, nil
}

// getDHCPHosts returns a array of used dhcp entry hosts
func (v *Virter) getDHCPHosts(network libvirt.Network) ([]libvirtxml.NetworkDHCPHost, error) {
	hosts := []libvirtxml.NetworkDHCPHost{}

	networkDescription, err := getNetworkDescription(v.libvirt, network)
	if err != nil {
		return hosts, err
	}
	if len(networkDescription.IPs) < 1 {
		return hosts, fmt.Errorf("no IPs in network")
	}

	ipDescription := networkDescription.IPs[0]

	dhcpDescription := ipDescription.DHCP
	if dhcpDescription == nil {
		return hosts, fmt.Errorf("no DHCP in network")
	}

	for _, host := range dhcpDescription.Hosts {
		hosts = append(hosts, host)
	}

	return hosts, nil
}

func (v *Virter) findIPs(network libvirt.Network, mac string) ([]string, error) {
	ips := []string{}

	hosts, err := v.getDHCPHosts(network)
	if err != nil {
		return ips, err
	}

	for _, host := range hosts {
		if host.MAC == mac {
			ips = append(ips, host.IP)
		}
	}

	return ips, nil
}

// cidr returns the network size of a oldstyle netmask. e.g.: 255.255.255.0 -> 24
func cidr(mask net.IP) uint {
	addr := mask.To4()
	sz, _ := net.IPv4Mask(addr[0], addr[1], addr[2], addr[3]).Size()
	return uint(sz)
}

// getVMID returns wantedID if it is not 0 and free.
// If wantedID is 0 getVMID searches for an unused ID and returns the first it can find
// For searching it uses the set libvirt network and already reserverd DHCP entries
func (v *Virter) getVMID(wantedID uint) (uint, error) {
	network, err := v.libvirt.NetworkLookupByName(v.networkName)
	if err != nil {
		return 0, fmt.Errorf("could not get network: %w", err)
	}

	hosts, err := v.getDHCPHosts(network)
	if err != nil {
		return 0, err
	}

	networkDescription, err := getNetworkDescription(v.libvirt, network)
	if err != nil {
		return 0, err
	}

	// get the network mask of the libvirt network
	_, ipNet, err := net.ParseCIDR(
		fmt.Sprintf("%s/%d", networkDescription.IPs[0].Address, cidr(net.ParseIP(networkDescription.IPs[0].Netmask))))
	if err != nil {
		return 0, err
	}

	// build a map of already used ID's
	usedIds := make(map[uint]bool, len(hosts))
	for _, host := range hosts {
		id, err := ipToID(*ipNet, net.ParseIP(host.IP))
		if err != nil {
			return 0, err
		}
		usedIds[id] = true
	}

	if wantedID != 0 { // one was already set
		if used := usedIds[wantedID]; used {
			return 0, fmt.Errorf("preset ID '%d' already used", wantedID)
		}
		// not used, we can hand it back
		return wantedID, nil
	}

	// try to find a free one

	// from the netmask the number of avialable hosts
	maskSize, _ := ipNet.Mask.Size()
	availableHosts := uint((1 << (32 - maskSize)) - 2)

	// we start from top of avialable host id's and check if they are already used and find one
	for i := availableHosts; i > 0; i-- {
		_, exists := usedIds[i]
		if !exists {
			return i, nil
		}
	}

	return 0, fmt.Errorf("could not find unused VM id")
}

func (v *Virter) tryReleaseDHCP(mac string, addrs []string, network libvirt.Network) error {
	networkDescription, err := getNetworkDescription(v.libvirt, network)
	if err != nil {
		return err
	}

	if networkDescription.Bridge == nil {
		return fmt.Errorf("network %q is not a bridge, cannot release dhcp", networkDescription.Name)
	}
	iface := networkDescription.Bridge.Name

	for _, addr := range addrs {
		log.Debugf("Releasing DHCP lease from %v to %v", mac, addr)
		cmd := exec.Command("sudo", "--non-interactive", "dhcp_release", iface, addr, mac)
		_, err = cmd.Output()
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				log.Debugf("dhcp_release stderr:\n%s", string(e.Stderr))
			}

			return fmt.Errorf("failed to run dhcp_release: %w", err)
		}
	}

	return nil
}
