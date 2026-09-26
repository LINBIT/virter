package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func imageTagCommand() *cobra.Command {
	tagCmd := &cobra.Command{
		Use:   "tag source target",
		Short: "Tag an image",
		Long:  `Create the image target, pointing to the same layers as the image source.`,
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			source := LocalImageName(args[0])
			target := LocalImageName(args[1])

			_, err = v.ImageTag(source, target, v.ProvisionStoragePool())
			if err != nil {
				log.WithError(err).Fatal("failed to tag image")
			}

			fmt.Printf("Tagged %s as %s\n", source, target)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// Only suggest first argument
				return suggestImageNames(cmd, args, toComplete)
			}

			return suggestNone(cmd, args, toComplete)
		},
	}

	return tagCmd
}
