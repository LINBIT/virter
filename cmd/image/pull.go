package image

import (
	"io"
	"log"
	"net/http"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"

	connect "github.com/LINBIT/virter/cmd"
)

// pullCmd represents the pull command
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull an image",
	Long: `Pull an image into a libvirt storage pool. If a URL is
explicitly given, the image will be fetched from there. Otherwise the
URL for the specified name from the local image registry will be
used.`,
	Run: imagePull,
}

var url string
var imageName string

func init() {
	imageCmd.AddCommand(pullCmd)

	pullCmd.Flags().StringVarP(&url, "url", "u", "", "URL to fetch from")
	pullCmd.Flags().StringVarP(&imageName, "name", "n", "", "name of image to create")
	pullCmd.MarkFlagRequired("name")
}

func imagePull(cmd *cobra.Command, args []string) {
	if url == "" {
		fillURLFromRegistry()
	}

	v, err := connect.VirterConnect()
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{}

	var total int64 = 0
	p := mpb.New()
	bar := p.AddBar(total,
		mpb.AppendDecorators(
			decor.CountersKibiByte("% .2f / % .2f"),
		),
	)

	err = v.ImagePull(
		client,
		BarReaderProxy{bar},
		url,
		imageName)
	if err != nil {
		log.Fatal(err)
	}
}

func fillURLFromRegistry() {
	var registry imageRegistry

	registryPath := viper.GetString("image.registry")

	_, err := toml.DecodeFile(registryPath, &registry)
	if err != nil {
		log.Fatal(err)
	}

	entry, ok := registry[imageName]
	if !ok {
		log.Fatalf("Image %v not found in registry and no URL given", imageName)
	}

	url = entry.URL
}

type imageEntry struct {
	URL string `toml:"url"`
}

type imageRegistry map[string]imageEntry

// BarReaderProxy adds the ReaderProxy methods to Bar
type BarReaderProxy struct {
	*mpb.Bar
}

// SetTotal sets the total for the bar
func (b BarReaderProxy) SetTotal(total int64) {
	b.Bar.SetTotal(total, false)
}

// ProxyReader wraps r so that the bar is updated as the data is read
func (b BarReaderProxy) ProxyReader(r io.ReadCloser) io.ReadCloser {
	return b.Bar.ProxyReader(r)
}
