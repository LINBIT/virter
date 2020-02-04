package internal

import (
	"fmt"
	"net/http"

	"github.com/digitalocean/go-libvirt"
	"github.com/google/uuid"
)

// HTTPClient contains required HTTP methods.
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// LibvirtConn contains required libvirt connection methods.
type LibvirtConn interface {
	StoragePoolLookupByName(Name string) (rPool libvirt.StoragePool, err error)
}

// ImagePull pulls an image from a URL into libvirt.
func ImagePull(conn LibvirtConn, client HTTPClient, url string) error {
	client.Get(url)

	sp, err := conn.StoragePoolLookupByName("images")
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	spUUID, err := uuid.FromBytes(sp.UUID[:])
	if err != nil {
		panic("failed to convert UUID from libvirt")
	}

	fmt.Printf("%v\n", spUUID)
	return nil
}
