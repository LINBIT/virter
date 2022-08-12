package virter

import (
	"fmt"
	"math/big"
	"net"
	"os/exec"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/digitalocean/go-libvirt"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
	log "github.com/sirupsen/logrus"
)

// AddDHCPHost determines the IP for an ID and adds a DHCP mapping from a MAC
// address to it. The same MAC address should always be paired with a given IP
// so that DHCP entries do not need to be released between removing a VM and
// creating another with the same ID.
func (v *Virter) AddDHCPHost(mac string, id uint) error {
	ipNet, err := v.getIPNet(v.provisionNetwork)
	if err != nil {
		return err
	}

	// Normalize network, as the IP returned is the one of the host interface by default
	ipNet.IP = ipNet.IP.Mask(ipNet.Mask)

	ip, err := cidr.Host(ipNet, int(id))
	if err != nil {
		return fmt.Errorf("failed to compute IP for network: %w", err)
	}

	log.Printf("Add DHCP entry from %v to %v", mac, ip)
	err = v.patchedNetworkUpdate(
		v.provisionNetwork,
		libvirt.NetworkUpdateCommandAddLast,
		libvirt.NetworkSectionIPDhcpHost,
		fmt.Sprintf("<host mac='%s' ip='%v'/>", mac, ip),
	)
	if err != nil {
		return fmt.Errorf("could not add DHCP entry: %w", err)
	}

	return nil
}

// Get the libvirt DNS server
func (v *Virter) getDNSServer() (net.IP, error) {
	ipNet, err := v.getIPNet(v.provisionNetwork)
	if err != nil {
		return nil, fmt.Errorf("could not get network description: %w", err)
	}

	return ipNet.IP, nil
}

// Get the domain suffix of the libvirt network. Returns an empty string if no
// domain is configured.
func (v *Virter) getDomainSuffix() (string, error) {
	net, err := getNetworkDescription(v.libvirt, v.provisionNetwork)
	if err != nil {
		return "", fmt.Errorf("could not get network xml: %w", err)
	}

	if net.Domain == nil || net.Domain.Name == "" {
		return "", nil
	}

	return net.Domain.Name, nil
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

func (v *Virter) getIPNet(network libvirt.Network) (*net.IPNet, error) {
	networkDescription, err := getNetworkDescription(v.libvirt, network)
	if err != nil {
		return nil, err
	}

	for i := range networkDescription.IPs {
		desc := networkDescription.IPs[i]
		if desc.Family != "" && desc.Family != "ipv4" {
			continue
		}

		if desc.Address == "" {
			return nil, fmt.Errorf("could not find address in network XML: '%v'", desc)
		}

		ip := net.ParseIP(desc.Address)
		if ip == nil {
			return nil, fmt.Errorf("could not parse network IP address '%s'", desc.Address)
		}

		if ip.To4() == nil {
			return nil, fmt.Errorf("not an IPv4 '%s'", ip)
		}

		if desc.Netmask == "" {
			return nil, fmt.Errorf("could not find netmask in network XML")
		}

		networkMaskIP := net.ParseIP(desc.Netmask)
		if networkMaskIP == nil {
			return nil, fmt.Errorf("could not parse network mask IP address")
		}

		networkMaskIPv4 := networkMaskIP.To4()
		if networkMaskIPv4 == nil {
			return nil, fmt.Errorf("network mask is not IPv4 address")
		}

		mask := net.IPMask(networkMaskIPv4)
		return &net.IPNet{IP: ip, Mask: mask}, nil
	}

	return nil, fmt.Errorf("no IPs in network")
}

// RemoveMACDHCPEntries removes DHCP host entries associated with the given
// MAC address
func (v *Virter) RemoveMACDHCPEntries(mac string) error {
	ips, err := v.findIPs(v.provisionNetwork, mac)
	if err != nil {
		return err
	}

	err = v.removeDHCPEntries(v.provisionNetwork, mac, ips)
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
			if hasErrorCode(err, libvirt.ErrNoNetwork) {
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
		err := v.patchedNetworkUpdate(
			network,
			libvirt.NetworkUpdateCommandDelete,
			libvirt.NetworkSectionIPDhcpHost,
			fmt.Sprintf("<host mac='%s' ip='%v'/>", mac, ip),
		)
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

// getDHCPHosts returns an array of used dhcp entry hosts
func (v *Virter) getDHCPHosts(network libvirt.Network) ([]libvirtxml.NetworkDHCPHost, error) {
	var hosts []libvirtxml.NetworkDHCPHost

	networkDescription, err := getNetworkDescription(v.libvirt, network)
	if err != nil {
		return hosts, err
	}

	for i := range networkDescription.IPs {
		desc := networkDescription.IPs[i]

		if desc.Family != "" && desc.Family != "ipv4" {
			continue
		}

		if desc.DHCP == nil {
			continue
		}

		hosts = append(hosts, desc.DHCP.Hosts...)
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

// GetVMID returns wantedID if it is not 0 and free.
// If wantedID is 0 GetVMID searches for an unused ID and returns the first it can find.
// For searching it uses the set libvirt network and already reserved DHCP entries.
func (v *Virter) GetVMID(wantedID uint, expectDHCPEntry bool) (uint, error) {
	if expectDHCPEntry {
		if wantedID == 0 {
			return 0, fmt.Errorf("ID must be set in static DHCP mode")
		}

		mac := QemuMAC(wantedID)
		ips, err := v.findIPs(v.provisionNetwork, mac)
		if err != nil {
			return 0, err
		}

		if len(ips) < 1 {
			return 0, fmt.Errorf("DHCP host entry for ID '%d' not found (static DHCP mode)", wantedID)
		}

		return wantedID, nil
	}

	ipnet, err := v.getIPNet(v.provisionNetwork)
	if err != nil {
		return 0, fmt.Errorf("failed to get access network: %w", err)
	}

	prefix, size := ipnet.Mask.Size()

	start := uint(2)
	end := (uint(1) << (size - prefix)) - 2

	if wantedID != 0 {
		end = wantedID
		start = wantedID
	}

	// we start from top of avialable host id's and check if they are already used and find one
	for i := end; i >= start; i-- {
		mac := QemuMAC(wantedID)
		ips, err := v.findIPs(v.provisionNetwork, mac)
		if err != nil {
			return 0, err
		}

		if len(ips) == 0 {
			return i, nil
		}
	}

	if wantedID != 0 {
		return 0, fmt.Errorf("preset ID '%d' already used", wantedID)
	} else {
		return 0, fmt.Errorf("could not find unused VM id")
	}
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
