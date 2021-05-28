package cmd

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func imageRmCommand() *cobra.Command {
	rmCmd := &cobra.Command{
		Use:   "rm name [name...]",
		Short: "Remove images",
		Long:  `Remove one or multiple images from a libvirt storage pool.`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			var errs error
			for _, vm := range args {
				err = v.ImageRm(context.Background(), vm)
				if err != nil {
					e := fmt.Errorf("failed to remove image '%s': %w", vm, err)
					errs = multierror.Append(errs, e)
				}
			}
			if errs != nil {
				log.Fatal(errs)
			}
		},
	}

	return rmCmd
}
