package virter

import (
	"fmt"
	"net/http"

	"github.com/digitalocean/go-libvirt"
)

// FileReader is the interface for reading whole files.
type FileReader interface {
	ReadFile(subpath string) ([]byte, error)
}

// HTTPClient contains required HTTP methods.
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// LibvirtConnection contains required libvirt connection methods.
type LibvirtConnection interface {
	StoragePoolLookupByName(Name string) (rPool libvirt.StoragePool, err error)
	StorageVolCreateXML(Pool libvirt.StoragePool, XML string, Flags libvirt.StorageVolCreateFlags) (rVol libvirt.StorageVol, err error)
}

// Virter manipulates libvirt for virter.
type Virter struct {
	libvirt   LibvirtConnection
	templates FileReader
}

// New configures a new Virter.
func New(libvirtConnection LibvirtConnection, fileReader FileReader) *Virter {
	return &Virter{
		libvirt:   libvirtConnection,
		templates: fileReader,
	}
}

// ImagePull pulls an image from a URL into libvirt.
func (v *Virter) ImagePull(client HTTPClient, url string) error {
	xml, err := v.templates.ReadFile("volume-image.xml")
	if err != nil {
		return fmt.Errorf("could not read template: %w", err)
	}

	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get from %v: %w", url, err)
	}
	defer response.Body.Close()

	sp, err := v.libvirt.StoragePoolLookupByName("images")
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	sv, err := v.libvirt.StorageVolCreateXML(sp, string(xml), 0)
	if err != nil {
		return fmt.Errorf("could not create storage volume: %w", err)
	}

	fmt.Printf("%v\n", sv.Name)
	return nil
}
