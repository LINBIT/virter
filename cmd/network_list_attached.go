package cmd

import (
	"github.com/rodaine/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func networkListAttachedCommand() *cobra.Command {
	listAttachedCmd := &cobra.Command{
		Use:   "list-attached <network-name>",
		Short: "List VMs attached to a network",
		Long:  `List VMs attached to a network. Includes IP address and hostname if available.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			vmnics, err := v.NetworkListAttached(args[0])
			if err != nil {
				log.Fatal(err)
			}

			tbl := table.New("VM", "MAC", "IP", "Hostname", "Host Device")
			for _, vmnic := range vmnics {
				tbl.AddRow(vmnic.VMName, vmnic.MAC, vmnic.IP, vmnic.HostName, vmnic.HostDevice)
			}
			tbl.Print()
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return suggestNetworkNames(cmd, args, toComplete)
			}

			return suggestNone(cmd, args, toComplete)
		},
	}

	return listAttachedCmd
}
