package cmd

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"
)

func imagePullCommand() *cobra.Command {
	pullCmd := &cobra.Command{
		Use:   "pull name [tag|url]",
		Short: "Pull an image",
		Long: `Pull an image into a libvirt storage pool. If a URL or
Docker tag is explicitly given, the image will be fetched from there.
Otherwise the URL for the specified name from the local image registry 
will be used.`,
		Args: cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			dest := args[0]
			source := args[0]
			if len(args) == 2 {
				source = args[1]
			}

			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			p := mpb.New()

			ctx, cancel := onInterruptWrap(context.Background())
			defer cancel()

			image, err := GetLocalImage(ctx, dest, source, v, PullPolicyAlways, DefaultProgressFormat(p))
			if err != nil {
				log.WithError(err).Fatal("failed to pull image")
			}

			p.Wait()

			fmt.Printf("Pulled %s\n", image.Name())
		},
	}

	return pullCmd
}
