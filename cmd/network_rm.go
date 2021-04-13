package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func networkRmCommand() *cobra.Command {
	rmCmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a network",
		Long:  `Remove the named network.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			err = v.NetworkRemove(args[0])
			if err != nil {
				log.Fatal(err)
			}
		},
	}
	return rmCmd
}
