package cmd

import (
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"

	"github.com/LINBIT/virter/internal/virter"
)

func imageLoadCommand() *cobra.Command {
	load := &cobra.Command{
		Use:   "load name [file]",
		Short: "Load an image",
		Long: `Load an image from a standalone qcow2 image. If the second argument is omitted, the image will be 
read from stdin`,
		Args: cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			image := LocalImageName(args[0])
			if err != nil {
				log.WithError(err).Fatal("error parsing destination image name")
			}

			importLayer, err := v.NewDynamicLayer("load-"+image, v.ProvisionStoragePool())
			if err != nil {
				log.WithError(err).Fatal("error creating import layer")
			}

			p := mpb.New(DefaultContainerOpt())

			var in io.Reader
			if len(args) == 1 {
				log.Info("loading from stdin")
				in = os.Stdin
			} else {
				f, err := os.Open(args[1])
				if err != nil {
					log.WithError(err).Fatal("failed to open input file")
				}
				defer f.Close()

				in = f

				// If we can't stat the file, still allow this to continue, could be some special file.
				stat, err := f.Stat()
				if err == nil {
					bar := DefaultProgressFormat(p).NewBar(image, "load", stat.Size())
					in = bar.ProxyReader(in)
				}
			}

			err = importLayer.Upload(in)
			if err != nil {
				log.WithError(err).Fatal("error uploading data to layer")
			}

			vl, err := importLayer.ToVolumeLayer(nil, virter.WithProgress(DefaultProgressFormat(p)))
			if err != nil {
				log.WithError(err).Fatal("error persisting imported layer")
			}

			_, err = v.MakeImage(image, vl)
			if err != nil {
				log.WithError(err).Fatal("error tagging volume layer for image")
			}

			p.Wait()
			fmt.Printf("Loaded %s\n", image)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// Only suggest first argument
				return suggestImageNames(cmd, args, toComplete)
			}

			// Allow files here
			return nil, cobra.ShellCompDirectiveDefault
		},
	}

	return load
}
