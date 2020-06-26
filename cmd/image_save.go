package cmd

import (
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/spf13/cobra"
)

func imageSaveCommand() *cobra.Command {
	var out string

	saveCmd := &cobra.Command{
		Use:   "save image",
		Short: "Save an image",
		Long:  `Save an image file either to stdout or to disk.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			imageName := args[0]

			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			var dest io.Writer
			if out != "" {
				file, err := os.Create(out)
				if err != nil {
					log.Fatal(err)
				}
				dest = file
				defer file.Close()
			} else {
				if terminal.IsTerminal(int(os.Stdout.Fd())) {
					log.Fatal("Refusing to output image to a terminal")
				}
				dest = os.Stdout
			}
			err = v.ImageSave(imageName, dest)
			if err != nil {
				log.Fatalf("Failed to save image: %v", err)
			}
		},
	}

	saveCmd.Flags().StringVarP(&out, "out", "o", "", "File to write the image to")

	return saveCmd
}
