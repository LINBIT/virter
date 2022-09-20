package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func vmHostKeyCommand() *cobra.Command {
	hostKeyCmd := &cobra.Command{
		Use:   "host-key vm_name",
		Short: "Get the host key for a VM",
		Long:  `Get the host key for a VM in the format of an OpenSSH known_hosts file.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			knownHostsText, err := v.VMGetKnownHosts(args[0])
			if err != nil {
				log.Fatal(err)
			}

			fmt.Print(knownHostsText)
		},
		ValidArgsFunction: suggestVmNames,
	}
	return hostKeyCmd
}
