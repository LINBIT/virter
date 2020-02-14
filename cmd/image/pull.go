package image

import (
	"io"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"

	"github.com/LINBIT/virter/internal/connect"
)

// pullCmd represents the pull command
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull an image from a URL",
	Long:  `Pull an image from a URL into a libvirt storage pool.`,
	Run:   imagePull,
}

var url string
var imageName string

func init() {
	imageCmd.AddCommand(pullCmd)

	pullCmd.Flags().StringVarP(&url, "url", "u", "", "URL to fetch from")
	pullCmd.MarkFlagRequired("url")
	pullCmd.Flags().StringVarP(&imageName, "name", "n", "", "name of image to create")
	pullCmd.MarkFlagRequired("image")
}

func imagePull(cmd *cobra.Command, args []string) {
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
