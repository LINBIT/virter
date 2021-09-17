package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func imagePruneCommand() *cobra.Command {
	pruneCmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune unreferenced image layers",
		Long:  `Prune all image layers not referenced by tag images or VMs`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

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

	return pruneCmd
}
