package cmd

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func imageRmCommand() *cobra.Command {
	rmCmd := &cobra.Command{
		Use:   "rm name",
		Short: "Remove an image",
		Long:  `Remove an image from a libvirt storage pool.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			err = v.ImageRm(context.Background(), args[0])
			if err != nil {
				log.Fatalf("Error removing image: %v", err)
			}
		},
	}

	return rmCmd
}
