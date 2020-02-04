package internal

import (
	"fmt"
	"net/http"

	"github.com/libvirt/libvirt-go"
)

// HTTPClient contains required HTTP methods.
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// LibvirtConnect contains required libvirt connection methods.
type LibvirtConnect interface {
	LookupStoragePoolByName(name string) (LibvirtStoragePool, error)
	NewStream(flags libvirt.StreamFlags) (LibvirtStream, error)
}

// LibvirtStoragePool contains required libvirt storage pool methods.
type LibvirtStoragePool interface {
	GetUUIDString() (string, error)
}

// LibvirtStream contains required libvirt stream methods.
type LibvirtStream interface {
}

// ImagePull pulls an image from a URL into libvirt.
func ImagePull(conn LibvirtConnect, client HTTPClient, url string) error {
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
