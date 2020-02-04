package internal

import (
	"fmt"
	"net/http"

	"github.com/digitalocean/go-libvirt"
)

// Directory contains required file reading methods.
type Directory interface {
	ReadFile(subpath string) ([]byte, error)
}

// HTTPClient contains required HTTP methods.
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// LibvirtConn contains required libvirt connection methods.
type LibvirtConn interface {
	StoragePoolLookupByName(Name string) (rPool libvirt.StoragePool, err error)
	StorageVolCreateXML(Pool libvirt.StoragePool, XML string, Flags libvirt.StorageVolCreateFlags) (rVol libvirt.StorageVol, err error)
}

// VirterConn manipulates libvirt for virter.
type VirterConn struct {
	conn      LibvirtConn
	templates Directory
}

// New configures a new VirterConn.
func New(conn LibvirtConn, directory Directory) *VirterConn {
	return &VirterConn{
		conn:      conn,
		templates: directory,
	}
}

// ImagePull pulls an image from a URL into libvirt.
func (v *VirterConn) ImagePull(client HTTPClient, url string) error {
	xml, err := v.templates.ReadFile("volume-image.xml")
	if err != nil {
		return fmt.Errorf("could not read template: %w", err)
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get from %v: %w", url, err)
	}
	defer resp.Body.Close()

	sp, err := v.conn.StoragePoolLookupByName("images")
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	sv, err := v.conn.StorageVolCreateXML(sp, string(xml), 0)
	if err != nil {
		return fmt.Errorf("could not create storage volume: %w", err)
	}

	fmt.Printf("%v\n", sv.Name)
	return nil
}
