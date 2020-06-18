package cmd

import (
	"github.com/spf13/cobra"
)

func imageCommand() *cobra.Command {
	imageCmd := &cobra.Command{
		Use:   "image",
		Short: "Image related subcommands",
		Long:  `Image related subcommands.`,
	}

	imageCmd.AddCommand(imageBuildCommand())
	imageCmd.AddCommand(imagePullCommand())
	imageCmd.AddCommand(imageLsCommand())
	imageCmd.AddCommand(imageRmCommand())

	return imageCmd
}
