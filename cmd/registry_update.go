package cmd

import (
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func registryUpdateCommand() *cobra.Command {
	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update the builtin image registry",
		Long: `Fetch the latest version of the shipped image registry
and write it to a local file.`,

		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			registryPath := filepath.Join(defaultRegistryPath(), "images.toml")
			err := fetchShippedRegistry(registryPath)
			if err != nil {
				log.Fatalf("Failed to fetch registry: %v", err)
			}

			log.Infof("Successfully updated registry")
		},
	}

	return updateCmd
}
