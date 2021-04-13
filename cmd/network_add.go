package cmd

import (
	libvirtxml "github.com/libvirt/libvirt-go-xml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"net"
)

func networkAddCommand() *cobra.Command {
	var forward string
	var dhcp bool
	var network string
	var domain string

	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new network",
		Long:  `Add a new network. VMs can be attached to such a network in addition to the default network used by virter.`,
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
			}

			var addressesDesc []libvirtxml.NetworkIP
			if network != "" {
				ip, n, err := net.ParseCIDR(network)
				if err != nil {
					log.Fatal(err)
				}

				var dhcpDesc *libvirtxml.NetworkDHCP
				if dhcp {
					start := nextIP(ip)
					end := previousIP(broadcastAddress(n))

					n.Network()
					dhcpDesc = &libvirtxml.NetworkDHCP{
						Ranges: []libvirtxml.NetworkDHCPRange{{Start: start.String(), End: end.String()}},
					}
				}

				addressesDesc = append(addressesDesc, libvirtxml.NetworkIP{
					Address: ip.String(),
					Netmask: net.IP(n.Mask).String(),
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

			desc := libvirtxml.Network{
				Name:    args[0],
				Forward: forwardDesc,
				IPs:     addressesDesc,
				Domain:  domainDesc,
			}

			err = v.NetworkAdd(desc)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	addCmd.Flags().StringVarP(&forward, "forward-mode", "m", "", "Set the forward mode, for example 'nat'")
	addCmd.Flags().StringVarP(&network, "network-cidr", "n", "", "Configure the network range (IPv4) in CIDR notation. The IP will be assigned to the host device.")
	addCmd.Flags().BoolVarP(&dhcp, "dhcp", "p", false, "Configure DHCP. Use together with '--network-cidr'. DHCP range is configured starting from --network-cidr+1 until the broadcast address")
	addCmd.Flags().StringVarP(&domain, "domain", "d", "", "Configure DNS names for the network")
	return addCmd
}

func nextIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	for j := len(dup) - 1; j >= 0; j-- {
		dup[j]++
		if dup[j] > 0 {
			break
		}
	}
	return dup
}

func previousIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	for j := len(dup) - 1; j >= 0; j-- {
		dup[j]--
		if dup[j] < 255 {
			break
		}
	}
	return dup
}

func broadcastAddress(ipnet *net.IPNet) net.IP {
	dup := make(net.IP, len(ipnet.IP))
	copy(dup, ipnet.IP)
	for i := range dup {
		dup[i] = dup[i] | ^ipnet.Mask[i]
	}
	return dup
}
