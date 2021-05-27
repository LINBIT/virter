package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"

	"github.com/LINBIT/virter/internal/virter"
)

func pullLegacyRegistry(ctx context.Context, v *virter.Virter, image, url string, p *mpb.Progress) (*virter.LocalImage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	bar := DefaultProgressFormat(p).NewBar(image, "pull", response.ContentLength)
	proxyResponse := bar.ProxyReader(response.Body)
	defer proxyResponse.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad http status: %v", response.Status)
	}

	return v.ImageImportFromReader(image, proxyResponse, virter.WithProgress(DefaultProgressFormat(p)))
}

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
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			image := args[0]
			var pull string
			if len(args) == 1 {
				reg := loadRegistry()

				u, err := reg.Lookup(image)
				if err != nil {
					log.WithError(err).Fatal("Unknown image, don't know where to pull from")
				}

				pull = u
			} else {
				pull = args[1]
			}

			parsed, err := url.Parse(pull)
			if err != nil {
				log.WithError(err).Fatal("could not parse pull url")
			}

			p := mpb.New()
			defer p.Wait()

			ctx, cancel := onInterruptWrap(context.Background())
			defer cancel()

			if parsed.Scheme == "http" || parsed.Scheme == "https" {
				_, err := pullLegacyRegistry(ctx, v, image, pull, p)
				if err != nil {
					log.WithError(err).Fatal("failed to pull legacy image")
				}
			} else {
				parsedRef, err := name.ParseReference(pull, name.WithDefaultRegistry(""))
				if err != nil {
					log.WithError(err).Fatalf("Could not parse reference %s", pull)
				}

				if parsedRef.Context().Name() == "" {
					log.Fatalf("%s does not contain a registry reference, don't know where to pull from", parsedRef.Name())
				}

				img, err := remote.Image(parsedRef, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
				if err != nil {
					log.WithError(err).Fatalf("Could not fetch image information for %s", parsedRef.Name())
				}

				_, err = v.ImageImport(image, img, virter.WithProgress(DefaultProgressFormat(p)))
				if err != nil {
					log.WithError(err).Fatal("failed to import image")
				}
			}

			fmt.Printf("Pulled %s\n", image)
		},
	}

	return pullCmd
}
