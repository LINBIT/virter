package internal

import (
	"fmt"
	"net/http"

	"github.com/LINBIT/virter/internal/libvirtinterfaces"
)

// HTTPClient contains required HTTP methods.
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// ImagePull pulls an image from a URL into libvirt.
func ImagePull(conn libvirtinterfaces.LibvirtConnect, client HTTPClient, url string) error {
	client.Get(url)

	sp, err := conn.LookupStoragePoolByName("images")
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	uuid, err := sp.GetUUIDString()
	if err != nil {
		return fmt.Errorf("could not get UUID: %w", err)
	}

	fmt.Printf("%v\n", uuid)
	return nil
}
