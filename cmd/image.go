package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"

	"github.com/LINBIT/virter/internal/virter"
)

type PullPolicy string

func (p *PullPolicy) String() string {
	return string(*p)
}

func (p *PullPolicy) Set(s string) error {
	switch strings.ToLower(s) {
	case strings.ToLower(string(PullPolicyNever)), strings.ToLower(string(PullPolicyIfNotExist)), strings.ToLower(string(PullPolicyAlways)):
		*p = PullPolicy(s)
		return nil
	default:
		return fmt.Errorf("unknown pull policy. [%s, %s, %s]", PullPolicyAlways, PullPolicyIfNotExist, PullPolicyNever)
	}
}

func (p *PullPolicy) Type() string {
	return "pullPolicy"
}

const (
	PullPolicyAlways     PullPolicy = "Always"
	PullPolicyIfNotExist PullPolicy = "IfNotExist"
	PullPolicyNever      PullPolicy = "Never"
)

// LocalImageName returns the local name for the user-supplied image name.
//
// If supplied with an image name without any registry location, this just returns the original string.
//
// If supplied with an image name with registry location (the registry.example.com in registry.example.com/image:foo),
// the registry information will be stripped. "/" and ":" will be replaced by "-".
//
// Examples:
// * local-image -> local-image
// * image:foo -> image:foo
// * registry.example.com/image:foo -> image-foo
// * registry.example.com/namespace/image -> namespace--image-latest
func LocalImageName(img string) string {
	parseImg, err := name.ParseReference(img, name.WithDefaultRegistry(""))
	if err != nil {
		log.WithError(err).Warnf("Image %s could not be parsed as a registry reference", img)
		return img
	}

	// Split the parsed reference in it's components. Using 'registry.example.com:5000/namespace/name:v1.2.3' as example,
	// we get:
	// * ident:    v1.2.3
	// * baseName: namespace/name
	// * registry: registry.example.com:5000
	ident := parseImg.Identifier()
	baseName := parseImg.Context().RepositoryStr()
	registry := parseImg.Context().RegistryStr()

	if registry == "" {
		return img
	}

	localName := fmt.Sprintf("%s-%s", strings.ReplaceAll(baseName, "/", "--"), ident)

	log.Infof("Mapping image %s to local volume %s", img, localName)

	return localName
}

// GetLocalImage tries to find the image in local storage.
//
// If the image, given by the imageName is not found in local storage, it is pulled from source.
// Image names are resolved according to LocalImageName. It is possible to specify:
// * A local image name.
// * A name matching an alias in the legacy image registry.
// * A container registry reference, which will be converted into a compatible local volume name.
func GetLocalImage(ctx context.Context, imageName string, source string, v *virter.Virter, policy PullPolicy, p virter.ProgressOpt) (*virter.LocalImage, error) {
	localName := LocalImageName(imageName)

	switch policy {
	case PullPolicyNever, PullPolicyIfNotExist:
		localImg, err := v.FindImage(localName, virter.WithProgress(p))
		if err != nil {
			return nil, err
		}

		if localImg != nil {
			return localImg, nil
		}

		if policy == PullPolicyNever {
			return nil, fmt.Errorf("image %s not present in local storage", imageName)
		}
		// PullPolicyIfNotExist -> try to pull
	case PullPolicyAlways:
		// Skip to pulling
	default:
		return nil, fmt.Errorf("unknown pull policy %s", policy)
	}

	isHttpUrl := strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")

	parsedRef, err := name.ParseReference(source, name.WithDefaultRegistry(""))
	if isHttpUrl || err != nil || parsedRef.Context().Registry.Name() == "" {
		log.Tracef("Source %s failed to parse or has no registry location, trying non-registry pull", source)
		return pullNonContainerRegistry(ctx, v, localName, source, p)
	}

	srcImg, err := remote.Image(parsedRef, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("could not fetch image information for %s: %w", parsedRef.Name(), err)
	}

	return v.ImageImport(localName, srcImg, virter.WithProgress(p))
}

// pullNonContainerRegistry tries to pull an image from a source.
//
// The source can be either a HTTP url or a alias in the built-in image registry.
func pullNonContainerRegistry(ctx context.Context, v *virter.Virter, destination, source string, p virter.ProgressOpt) (*virter.LocalImage, error) {
	parsedSource, err := url.Parse(source)
	if err != nil || (parsedSource.Scheme != "http" && parsedSource.Scheme != "https") {
		registry := loadRegistry()

		registryUrl, err := registry.Lookup(source)
		if err != nil {
			return nil, fmt.Errorf("failed to look up %s: %w", source, err)
		}

		parsedSource, err = url.Parse(registryUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to look up %s: %w", source, err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedSource.String(), nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	bar := p.NewBar(destination, "pull", response.ContentLength)
	proxyResponse := bar.ProxyReader(response.Body)
	defer proxyResponse.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad http status: %v", response.Status)
	}

	return v.ImageImportFromReader(destination, proxyResponse, virter.WithProgress(p))
}

func imageCommand() *cobra.Command {
	imageCmd := &cobra.Command{
		Use:   "image",
		Short: "Image related subcommands",
		Long:  `Image related subcommands.`,
	}

	imageCmd.AddCommand(imageBuildCommand())
	imageCmd.AddCommand(imagePullCommand())
	imageCmd.AddCommand(imageLsCommand())
	imageCmd.AddCommand(imageRmCommand())
	imageCmd.AddCommand(imageLoadCommand())
	imageCmd.AddCommand(imageSaveCommand())
	imageCmd.AddCommand(imagePushCommand())
	imageCmd.AddCommand(imagePruneCommand())

	return imageCmd
}

type mpbProgress struct {
	*mpb.Progress
}

func DefaultProgressFormat(p *mpb.Progress) virter.ProgressOpt {
	return &mpbProgress{Progress: p}
}

func (m *mpbProgress) NewBar(name, operation string, total int64) *mpb.Bar {
	if len(name) > 24 {
		name = name[:24]
	}

	return m.Progress.AddBar(
		total,
		mpb.PrependDecorators(
			decor.Name(name, decor.WC{W: len(name) + 1, C: decor.DidentRight}),
			decor.OnComplete(decor.Name(operation, decor.WCSyncWidthR), fmt.Sprintf("%s done", operation)),
		),
		mpb.AppendDecorators(decor.CountersKibiByte("%.2f / %.2f")),
	)
}
