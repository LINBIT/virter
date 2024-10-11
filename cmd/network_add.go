package cmd

import (
	"net"

	"github.com/apparentlymart/go-cidr/cidr"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/internal/virter"
)

func networkAddCommand() *cobra.Command {
	var forward string
	var dhcp bool
	var dhcpMAC string
	var dhcpID uint
	var dhcpCount uint
	var network string
	var networkV6 string
	var domain string

	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new network",
		Long:  `Add a new network. VMs can be attached to such a network in addition to the default network used by virter. DHCP entries can be added directly to the new network.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			var forwardDesc *libvirtxml.NetworkForward
			if forward != "" {
				forwardDesc = &libvirtxml.NetworkForward{
					Mode: forward,
				}

				if forward == "nat" && networkV6 != "" {
					forwardDesc.NAT = &libvirtxml.NetworkForwardNAT{
						IPv6: "yes",
					}
				}
			}

			var addressesDesc []libvirtxml.NetworkIP
			if network != "" {
				ip, n, err := net.ParseCIDR(network)
				if err != nil {
					log.Fatal(err)
				}

				var dhcpDesc *libvirtxml.NetworkDHCP
				if dhcp {
					dhcpDesc = buildNetworkDHCP(ip, n, dhcpMAC, dhcpID, dhcpCount)
				}

				addressesDesc = append(addressesDesc, libvirtxml.NetworkIP{
					Address: ip.String(),
					Netmask: net.IP(n.Mask).String(),
					DHCP:    dhcpDesc,
				})
			}

			if networkV6 != "" {
				ip, n, err := net.ParseCIDR(networkV6)
				if err != nil {
					log.Fatal(err)
				}

				var dhcpDesc *libvirtxml.NetworkDHCP
				if dhcp {
					// DHCPv6 is complicated, especially since you can't use the mac address of a VM to assign a fixed
					// IP. so disable that for now
					dhcpDesc = buildNetworkDHCP(ip, n, dhcpMAC, dhcpID, 0)
				}

				prefix, _ := n.Mask.Size()
				addressesDesc = append(addressesDesc, libvirtxml.NetworkIP{
					Family:  "ipv6",
					Address: ip.String(),
					Prefix:  uint(prefix),
					DHCP:    dhcpDesc,
				})
			}

			var domainDesc *libvirtxml.NetworkDomain
			if domain != "" {
				domainDesc = &libvirtxml.NetworkDomain{
					Name:      domain,
					LocalOnly: "yes",
				}
			}

			var dnsDesc *libvirtxml.NetworkDNS
			if domain == "" && forward == "" {
				dnsDesc = &libvirtxml.NetworkDNS{
					Enable: "no",
				}
			}

			var options []libvirtxml.NetworkDnsmasqOption
			for _, opt := range viper.GetStringSlice("libvirt.dnsmasq_options") {
				options = append(options, libvirtxml.NetworkDnsmasqOption{Value: opt})
			}

			desc := libvirtxml.Network{
				Name:    args[0],
				Forward: forwardDesc,
				IPs:     addressesDesc,
				Domain:  domainDesc,
				DNS:     dnsDesc,
				DnsmasqOptions: &libvirtxml.NetworkDnsmasqOptions{
					Option: options,
				},
			}

			err = v.NetworkAdd(desc)
			if err != nil {
				log.Fatal(err)
			}
		},
		ValidArgsFunction: suggestNone,
	}

	addCmd.Flags().StringVarP(&forward, "forward-mode", "m", "", "Set the forward mode, for example 'nat'")
	addCmd.Flags().StringVarP(&network, "network-cidr", "n", "", "Configure the network range (IPv4) in CIDR notation. The IP will be assigned to the host device.")
	addCmd.Flags().StringVarP(&networkV6, "network-v6-cidr", "6", "", "Configure the network range (IPv6) in CIDR notation. The IP will be assigned to the host device.")
	addCmd.Flags().BoolVarP(&dhcp, "dhcp", "p", false, "Configure DHCP. Use together with '--network-cidr'. DHCP range is configured starting from --network-cidr+1 until the broadcast address")
	addCmd.Flags().StringVarP(&dhcpMAC, "dhcp-mac", "", virter.QemuBaseMAC().String(), "Base MAC address to which ID is added. The default can be used to populate a virter access network")
	addCmd.Flags().UintVarP(&dhcpID, "dhcp-id", "", 0, "ID which determines the MAC and IP addresses to associate")
	addCmd.Flags().UintVar(&dhcpCount, "dhcp-count", 0, "Number of host entries to add")
	addCmd.Flags().StringVarP(&domain, "domain", "d", "", "Configure DNS names for the network")
	return addCmd
}

func buildNetworkDHCP(ip net.IP, n *net.IPNet, dhcpMAC string, dhcpID uint, dhcpCount uint) *libvirtxml.NetworkDHCP {
	start := cidr.Inc(ip)
	_, end := cidr.AddressRange(n)

	prefix, hostBits := n.Mask.Size()
	// libvirt does not support more than 2^16-1 hosts in the dhcp range
	if hostBits-prefix > 16 {
		end, _ = cidr.Host(n, 65535)
	} else {
		end = cidr.Dec(end)
	}

	baseMAC, err := net.ParseMAC(dhcpMAC)
	if err != nil {
		log.Fatal(err)
	}

	current := start
	hosts := make([]libvirtxml.NetworkDHCPHost, dhcpCount)
	for i := uint(0); i < dhcpCount; i++ {
		id := dhcpID + i
		mac, err := virter.AddToMAC(baseMAC, id)
		if err != nil {
			log.Fatal(err)
		}

		hosts[i].MAC = mac.String()
		hosts[i].IP = current.String()
		current = cidr.Inc(current)
	}

	return &libvirtxml.NetworkDHCP{
		Ranges: []libvirtxml.NetworkDHCPRange{{Start: start.String(), End: end.String()}},
		Hosts:  hosts,
	}
}
