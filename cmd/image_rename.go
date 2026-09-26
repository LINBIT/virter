package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func imageRenameCommand() *cobra.Command {
	renameCmd := &cobra.Command{
		Use:   "rename source target",
		Short: "Rename an image",
		Long:  `Rename the image source to target.`,
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			source := LocalImageName(args[0])
			target := LocalImageName(args[1])

			pool := v.ProvisionStoragePool()

			_, err = v.ImageTag(source, target, pool)
			if err != nil {
				log.WithError(err).Fatal("failed to rename image")
			}

			if source != target {
				err = v.ImageRm(source, pool)
				if err != nil {
					log.WithError(err).Fatalf("created %s, but failed to remove %s", target, source)
				}
			}

			fmt.Printf("Renamed %s to %s\n", source, target)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// Only suggest first argument
				return suggestImageNames(cmd, args, toComplete)
			}

			return suggestNone(cmd, args, toComplete)
		},
	}

	return renameCmd
}
