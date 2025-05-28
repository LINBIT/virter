package cmd

import (
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func imagePruneCommand() *cobra.Command {
	var deleteUnusedForDuration time.Duration

	pruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune unreferenced or unused image layers",
		Long:  `Prune all image layers not referenced by tag images or VMs`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			if deleteUnusedForDuration > 0 {
				now := time.Now()

				images, err := v.ImageList()
				if err != nil {
					log.WithError(err).Fatal("failed to get image list")
				}

				for _, image := range images {
					vol, err := image.TopLayer().Descriptor()
					if err != nil {
						log.WithError(err).Fatalf("failed to get tag volume for '%s'", image.Name())
					}

					atime, err := strconv.ParseFloat(vol.Target.Timestamps.Atime, 64)
					if err != nil {
						log.WithError(err).Fatalf("failed to parse atime for volume '%s'", image.Name())
					}

					if time.Unix(int64(atime), 0).Add(deleteUnusedForDuration).Before(now) {
						err := v.ImageRm(image.Name(), v.ProvisionStoragePool())
						if err != nil {
							log.WithError(err).Fatalf("failed to delete image '%s'", image.Name())
						}

						log.WithField("image", image.Name()).Info("deleted image")
					}
				}
			}

			layers, err := v.LayerList()
			if err != nil {
				log.WithError(err).Fatal("failed to get layer list")
			}

			// In theory, we would build a proper dependency graph to figure out right deletion order.
			// In practice, we just iterate over all layers and try to delete what we can. To handle
			// cases where we have layer a depending on b, and we try to delete b first (which will not succeed)
			// we just follow the dependency chain once we have a successful deletion. So in the example, after
			// we deleted a, we would follow the chain and try to delete b again.
			for _, layer := range layers {
				err := layer.DeleteAllIfUnused()
				if err != nil {
					log.WithError(err).Warn("could not prune layer")
				}
			}
		},
		ValidArgsFunction: suggestNone,
	}

	pruneCmd.Flags().DurationVar(&deleteUnusedForDuration, "delete-unused", 0, "delete images that have not been used for the given duration")

	return pruneCmd
}
