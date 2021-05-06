package virter

import (
	"fmt"
	"math/big"
	"net"
	"os/exec"

	libvirt "github.com/digitalocean/go-libvirt"
	log "github.com/sirupsen/logrus"

	libvirtxml "github.com/libvirt/libvirt-go-xml"
)

// AddDHCPHost determines the IP for an ID and adds a DHCP mapping from a MAC
// address to it. The same MAC address should always be paired with a given IP
// so that DHCP entries do not need to be released between removing a VM and
// creating another with the same ID.
func (v *Virter) AddDHCPHost(mac string, id uint) error {
	network, err := v.libvirt.NetworkLookupByName(v.networkName)
	if err != nil {
		return fmt.Errorf("could not get network: %w", err)
	}

	ipNet, err := v.getIPNet(network)
	if err != nil {
		return err
	}

	networkBaseIP := ipNet.IP.Mask(ipNet.Mask)
	ip := AddToIP(networkBaseIP, id)

	if !ipNet.Contains(ip) {
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

// Get the libvirt DNS server
func (v *Virter) getDNSServer() (net.IP, error) {
	network, err := v.libvirt.NetworkLookupByName(v.networkName)
	if err != nil {
		return nil, fmt.Errorf("could not get network: %w", err)
	}

	ipNet, err := v.getIPNet(network)
	if err != nil {
		return nil, fmt.Errorf("could not get network description: %w", err)
	}

	return ipNet.IP, nil
}

// Get the domain suffix of the libvirt network. Returns an empty string if no
// domain is configured.
func (v *Virter) getDomainSuffix() (string, error) {
	network, err := v.libvirt.NetworkLookupByName(v.networkName)
	if err != nil {
		return "", fmt.Errorf("could not get network: %w", err)
	}

	net, err := getNetworkDescription(v.libvirt, network)
	if err != nil {
		return "", fmt.Errorf("could not get network xml: %w", err)
	}

	if net.Domain == nil || net.Domain.Name == "" {
		return "", nil
	}

	return net.Domain.Name, nil
}

func AddToIP(ip net.IP, addend uint) net.IP {
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

// QemuMAC calculates a MAC address for a given id
func QemuMAC(id uint) string {
	mac, err := AddToMAC(QemuBaseMAC(), id)
	if err != nil {
		// Should never happen because "id" should never exceed 32 bits
		// and any 32 bit value can be added to the QEMU base MAC.
		panic(fmt.Sprintf("failed to construct QEMU MAC: %v", err))
	}

	return mac.String()
}

func QemuBaseMAC() net.HardwareAddr {
	mac, err := net.ParseMAC("52:54:00:00:00:00")
	if err != nil {
		panic("failed to parse hardcoded MAC address")
	}

	return mac
}

func AddToMAC(mac net.HardwareAddr, addend uint) (net.HardwareAddr, error) {
	var value big.Int
	value.SetBytes(mac)
	value.Add(&value, big.NewInt(int64(addend)))

	valueBytes := value.Bytes()
	if len(valueBytes) > len(mac) {
		return net.HardwareAddr{}, fmt.Errorf("overflow adding %d to %v", addend, mac)
	}

	// zero-pad bytes
	out := make([]byte, len(mac))
	copy(out[len(out)-len(valueBytes):], valueBytes)
	return out, nil
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

// RemoveMACDHCPEntries removes DHCP host entries associated with the given
// MAC address
func (v *Virter) RemoveMACDHCPEntries(mac string) error {
	network, err := v.libvirt.NetworkLookupByName(v.networkName)
	if err != nil {
		return fmt.Errorf("could not get network: %w", err)
	}

	ips, err := v.findIPs(network, mac)
	if err != nil {
		return err
	}

	err = v.removeDHCPEntries(network, mac, ips)
	if err != nil {
		return err
	}

	return nil
}

func (v *Virter) removeDomainDHCP(domain libvirt.Domain, removeDHCPEntries bool) error {
	nics, err := v.getNICs(domain)
	if err != nil {
		return err
	}

	for _, nic := range nics {
		if nic.Network == "" {
			continue
		}

		network, err := v.libvirt.NetworkLookupByName(nic.Network)
		if err != nil {
			if hasErrorCode(err, errNoNetwork) {
				// We ignore non-existing networks, as there is no dhcp entry to remove
				continue
			}

			return fmt.Errorf("could not get network: %w", err)
		}

		ips, err := v.findIPs(network, nic.MAC)
		if err != nil {
			return err
		}

		if removeDHCPEntries {
			err = v.removeDHCPEntries(network, nic.MAC, ips)
			if err != nil {
				return err
			}
		}

		err = v.tryReleaseDHCP(network, nic.MAC, ips)
		if err != nil {
			log.Debugf("Could not release DHCP lease: %v", err)
		}
	}

	return nil
}

func (v *Virter) removeDHCPEntries(network libvirt.Network, mac string, ips []string) error {
	for _, ip := range ips {
		log.Printf("Remove DHCP entry from %v to %v", mac, ip)
		err := v.libvirt.NetworkUpdate(
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

	return nil
}

func (v *Virter) getDomainForMAC(mac string) (libvirt.Domain, error) {
	domains, _, err := v.libvirt.ConnectListAllDomains(-1, 0)
	if err != nil {
		return libvirt.Domain{}, fmt.Errorf("could not list domains: %w", err)
	}

	for _, domain := range domains {
		nics, err := v.getNICs(domain)
		if getErr, ok := err.(*LibvirtGetError); ok && getErr.NotFound {
			continue
		}
		if err != nil {
			return libvirt.Domain{}, fmt.Errorf("could not check MAC for domain '%s': %w", domain.Name, err)
		}
		for _, nic := range nics {
			if nic.MAC == mac {
				return domain, nil
			}
		}
	}

	return libvirt.Domain{}, nil
}

type nic struct {
	MAC        string
	Network    string
	HostDevice string
}

// getNICs returns the list of macs and their virtual network
func (v *Virter) getNICs(domain libvirt.Domain) ([]nic, error) {
	domainDescription, err := getDomainDescription(v.libvirt, domain)
	if err != nil {
		return nil, err
	}

	devicesDescription := domainDescription.Devices
	if devicesDescription == nil {
		return nil, fmt.Errorf("no devices in domain")
	}

	var nics []nic
	for _, interfaceDescription := range devicesDescription.Interfaces {
		mac := ""
		if interfaceDescription.MAC != nil {
			mac = interfaceDescription.MAC.Address
		}

		network := ""
		if interfaceDescription.Source != nil && interfaceDescription.Source.Network != nil {
			network = interfaceDescription.Source.Network.Network
		}

		hostdevice := ""
		if interfaceDescription.Target != nil {
			hostdevice = interfaceDescription.Target.Dev
		}

		nics = append(nics, nic{
			MAC:        mac,
			Network:    network,
			HostDevice: hostdevice,
		})
	}

	return nics, nil
}

// getDHCPHosts returns a array of used dhcp entry hosts
func (v *Virter) getDHCPHosts(network libvirt.Network) ([]libvirtxml.NetworkDHCPHost, error) {
	hosts := []libvirtxml.NetworkDHCPHost{}

	networkDescription, err := getNetworkDescription(v.libvirt, network)
	if err != nil {
		return hosts, err
	}
	if len(networkDescription.IPs) < 1 {
		// No IPs == no DHCP entries
		return hosts, nil
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

// GetVMID returns wantedID if it is not 0 and free.
// If wantedID is 0 GetVMID searches for an unused ID and returns the first it can find.
// For searching it uses the set libvirt network and already reserved DHCP entries.
func (v *Virter) GetVMID(wantedID uint, expectDHCPEntry bool) (uint, error) {
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

	if expectDHCPEntry {
		if wantedID == 0 {
			return 0, fmt.Errorf("ID must be set in static DHCP mode")
		}

		if !usedIds[wantedID] {
			return 0, fmt.Errorf("DHCP host entry for ID '%d' not found (static DHCP mode)", wantedID)
		}

		return wantedID, nil
	}

	if wantedID != 0 { // one was already set
		if usedIds[wantedID] {
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

func (v *Virter) tryReleaseDHCP(network libvirt.Network, mac string, addrs []string) error {
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
