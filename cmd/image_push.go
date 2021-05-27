package cmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"

	"github.com/LINBIT/virter/internal/virter"
)

func imagePushCommand() *cobra.Command {
	pushCmd := &cobra.Command{
		Use:     "push name repository-reference",
		Short:   "Push an image",
		Long:    `Push an image to a container registry.`,
		Example: "virter image push local-vm-image my.registry.org/namespace/name:latest",
		Args:    cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := onInterruptWrap(context.Background())
			defer cancel()

			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			p := mpb.NewWithContext(ctx)

			img, err := v.FindImage(args[0], virter.WithProgress(DefaultProgressFormat(p)))
			if err != nil {
				log.WithError(err).Fatal("failed to get image")
			}

			if img == nil {
				log.Fatalf("Unknown image %s", args[0])
			}

			ref, err := name.ParseReference(args[1], name.WithDefaultRegistry(""))
			if err != nil {
				log.WithError(err).Fatal("failed to parse destination ref")
			}

			err = remote.CheckPushPermission(ref, authn.DefaultKeychain, http.DefaultTransport)
			if err != nil {
				log.WithError(err).Fatal("not allowed to push")
			}

			err = remote.Write(
				ref,
				img,
				remote.WithAuthFromKeychain(authn.DefaultKeychain),
				remote.WithContext(ctx),
			)

			p.Wait()

			if err != nil {
				log.WithError(err).Fatal("failed to push image")
			}

			fmt.Printf("Pushed %s\n", ref.Name())
		},
	}

	return pushCmd
}
