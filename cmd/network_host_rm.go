package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/LINBIT/virter/internal/virter"

	"github.com/spf13/cobra"
)

func networkHostRmCommand() *cobra.Command {
	var vmID uint
	var count uint

	rmCmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove network host entries",
		Long:  `Remove one or more network host entries.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			if vmID == 0 {
				log.Fatalln("ID must be positive")
			}

			var i uint

			for i = 0; i < count; i++ {
				id := vmID + i

				mac := virter.QemuMAC(id)
				err := v.RemoveMACDHCPEntries(mac)
				if err != nil {
					log.Fatal(err)
				}
			}
		},
	}

	rmCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID which determines the host entry or entries to remove")
	rmCmd.MarkFlagRequired("id")
	rmCmd.Flags().UintVar(&count, "count", 1, "Number of IDs to deallocate")

	return rmCmd
}
