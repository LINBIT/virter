package cmd

import (
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"
	"golang.org/x/term"

	"github.com/LINBIT/virter/pkg/pullpolicy"
)

func imageSaveCommand() *cobra.Command {
	saveCmd := &cobra.Command{
		Use:   "save name [file]",
		Short: "Save an image",
		Long: `Saves an image as a standalone qcow2 image. If the image uses multiple layers, all layers will be
squashed into a single file.`,
		Args: cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			image := args[0]
			var out io.Writer
			if len(args) == 1 {
				out = os.Stdout
				if term.IsTerminal(int(os.Stdout.Fd())) {
					log.Fatal("refusing to write image to terminal, redirect or specify a filename")
				}
			} else {
				f, err := os.Create(args[1])
				if err != nil {
					log.WithError(err).Fatal("failed to create output file")
				}
				defer func() {
					_ = f.Close()
					if ctx.Err() != nil {
						_ = os.Remove(args[1])
					}
				}()

				out = f
			}

			p := mpb.NewWithContext(ctx, DefaultContainerOpt())

			imgRef, err := GetLocalImage(ctx, image, image, v, pullpolicy.Never, DefaultProgressFormat(p))
			if err != nil {
				log.WithError(err).Fatalf("error searching image %s", image)
			}

			if imgRef == nil {
				log.Fatalf("could not find a local image %s", image)
			}

			top := imgRef.TopLayer()

			squashed, err := top.Squashed()
			if err != nil {
				log.WithError(err).Fatal("could not squash image")
			}

			defer func() {
				err := squashed.Delete()
				if err != nil {
					log.WithError(err).Warn("failed to delete squashed volume")
				}
			}()

			desc, err := squashed.Descriptor()
			if err != nil {
				log.WithError(err).Fatal("could not get description of squashed volume")
			}

			bar := DefaultProgressFormat(p).NewBar(imgRef.Name(), "save", int64(desc.Physical.Value))

			reader, err := squashed.Uncompressed()
			if err != nil {
				log.WithError(err).Fatal("could not get reader from volume")
			}

			reader = bar.ProxyReader(reader)
			defer reader.Close()

			_, err = io.Copy(out, reader)
			if err != nil {
				log.WithError(err).Fatal("failed to copy volume content to output")
			}

			p.Wait()
			_, _ = fmt.Fprintf(os.Stderr, "Saved %s\n", imgRef.Name())
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

	return saveCmd
}
