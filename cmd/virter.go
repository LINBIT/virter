package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/directory"
)

func main() {
	err := imagePull()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func imagePull() error {
	var templates directory.Directory = "assets/libvirt-templates"

	c, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 2*time.Second)
	if err != nil {
		return fmt.Errorf("failed to dial libvirt: %w", err)
	}

	l := libvirt.New(c)
	if err := l.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	v := virter.New(l, "images", templates)

	client := &http.Client{}

	var total int64 = 0
	p := mpb.New()
	bar := p.AddBar(total,
		mpb.AppendDecorators(
			decor.CountersKibiByte("% .2f / % .2f"),
		),
	)

	return v.ImagePull(
		client,
		BarReaderProxy{bar},
		"https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img",
		"some-name")
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
