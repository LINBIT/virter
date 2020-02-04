package internal

import (
	"net/http"
)

// HTTPClient contains required HTTP methods.
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// ImagePull pulls an image from a URL into libvirt.
func ImagePull(client HTTPClient, url string) error {
	client.Get(url)
	return nil
}
