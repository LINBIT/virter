package cmd

import (
	"fmt"
	"github.com/rodaine/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"net"
	"strings"

	"github.com/spf13/cobra"
)

func networkLsCommand() *cobra.Command {
	lsCmd := &cobra.Command{
		Use:   "ls",
		Short: "List available networks",
		Long:  `List available networks that VMs can be attached to.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			xmls, err := v.NetworkList()
			if err != nil {
				log.Fatal(err)
			}

			virterNet := viper.GetString("libvirt.network")

			tbl := table.New("Name", "Forward-Type", "IP-Range", "Domain", "DHCP", "Bridge")
			for _, desc := range xmls {
				name := desc.Name
				if name == virterNet {
					name = fmt.Sprintf("%s (virter default)", name)
				}
				ty := ""
				if desc.Forward != nil {
					ty = desc.Forward.Mode
				}

				var ranges []string
				netrg := make([]string, len(desc.IPs))
				for i, n := range desc.IPs {
					ip := net.ParseIP(n.Address)
					mask := net.ParseIP(n.Netmask)
					netrg[i] = (&net.IPNet{IP: ip, Mask: net.IPMask(mask)}).String()

					if n.DHCP != nil {
						for _, r := range n.DHCP.Ranges {
							ranges = append(ranges, fmt.Sprintf("%s-%s", r.Start, r.End))
						}
					}
				}
				netranges := strings.Join(netrg, ",")

				domain := ""
				if desc.Domain != nil {
					domain = desc.Domain.Name
				}

				dhcp := strings.Join(ranges, ",")

				bridge := ""
				if desc.Bridge != nil {
					bridge = desc.Bridge.Name
				}

				tbl.AddRow(name, ty, netranges, domain, dhcp, bridge)
			}

			tbl.Print()
		},
	}

	return lsCmd
}
