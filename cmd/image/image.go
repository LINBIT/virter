package image

import (
	"github.com/spf13/cobra"

	"github.com/LINBIT/virter/cmd"
)

// imageCmd represents the image command
var imageCmd = &cobra.Command{
	Use:   "image",
	Short: "Image related subcommands",
	Long:  `Image related subcommands.`,
}

func init() {
	cmd.RootCmd.AddCommand(imageCmd)
}
