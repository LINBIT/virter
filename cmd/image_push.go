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
)

func imagePushCommand() *cobra.Command {
	pushCmd := &cobra.Command{
		Use:     "push name [repository-reference]",
		Short:   "Push an image",
		Long:    `Push an image to a container registry. If one argument is given, it is the push target; the local image to be pushed is inferred. When using two arguments, the first is the local image name, the second the push location.`,
		Example: "virter image push my.registry.org/namespace/name:latest",
		Args:    cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			source := args[0]
			dest := args[0]
			if len(args) == 2 {
				dest = args[1]
			}

			ctx, cancel := onInterruptWrap(context.Background())
			defer cancel()

			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			p := mpb.NewWithContext(ctx)

			img, err := GetLocalImage(ctx, source, source, v, PullPolicyNever, DefaultProgressFormat(p))
			if err != nil {
				log.WithError(err).Fatal("failed to get image")
			}

			if img == nil {
				log.Fatalf("Unknown image %s", args[0])
			}

			ref, err := name.ParseReference(dest, name.WithDefaultRegistry(""))
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
