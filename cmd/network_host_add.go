package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/LINBIT/virter/internal/virter"

	"github.com/spf13/cobra"
)

func networkHostAddCommand() *cobra.Command {
	var vmID uint
	var count uint

	addCmd := &cobra.Command{
		Use:   "add",
		Short: "Add network host entries",
		Long:  `Add one or more network host entries.`,
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

				_, err := v.GetVMID(id)
				if err != nil {
					log.Fatal(err)
				}
			}

			for i = 0; i < count; i++ {
				id := vmID + i

				mac := virter.QemuMAC(id)
				_, err := v.AddDHCPEntry(mac, id)
				if err != nil {
					log.Fatal(err)
				}
			}
		},
	}

	addCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID which determines the MAC and IP addresses to associate")
	addCmd.MarkFlagRequired("id")
	addCmd.Flags().UintVar(&count, "count", 1, "Number of IDs to allocate")

	return addCmd
}
