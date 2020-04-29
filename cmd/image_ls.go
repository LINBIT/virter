package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

var listAvailable bool

func imageLsCommand() *cobra.Command {
	lsCmd := &cobra.Command{
		Use:   "ls",
		Short: "List images",
		Long:  `List all images available locally.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if listAvailable {
				reg := loadRegistry()
				entries, err := reg.List()
				if err != nil {
					log.Fatalf("Error listing images: %v", err)
				}

				for name, entry := range entries {
					fmt.Printf("%s: %s\n", name, entry.URL)
				}
			} else {
				log.Fatalf("Listing locally available image files is not supported yet")
			}
		},
	}

	lsCmd.Flags().BoolVar(&listAvailable, "available", false, "List all images available from image registries")

	return lsCmd
}
