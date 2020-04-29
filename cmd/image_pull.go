package cmd

import (
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
)

func imagePullCommand() *cobra.Command {
	var url string

	pullCmd := &cobra.Command{
		Use:   "pull name",
		Short: "Pull an image",
		Long: `Pull an image into a libvirt storage pool. If a URL is
explicitly given, the image will be fetched from there. Otherwise the
URL for the specified name from the local image registry will be
used.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			reg := loadRegistry()
			imageName := args[0]
			if url == "" {
				var err error
				url, err = reg.Lookup(imageName)
				if err != nil {
					log.Fatalf("Error pulling image: %v", err)
				}
			}

			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}
			defer v.Disconnect()

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
		},
	}

	pullCmd.Flags().StringVarP(&url, "url", "u", "", "URL to fetch from")

	return pullCmd
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
