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

	imageCmd.AddCommand(imagePullCommand())

	return imageCmd
}
