package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information of virter",
		Run: func(cmd *cobra.Command, args []string) {
			if version == "" {
				version = "DEV"
			}
			if builddate == "" {
				builddate = "DEV"
			}
			if githash == "" {
				githash = "DEV"
			}
			fmt.Printf("virter version %s\n", version)
			fmt.Printf("Built at %s\n", builddate)
			fmt.Printf("Version control hash: %s\n", githash)

		},
	}
}
