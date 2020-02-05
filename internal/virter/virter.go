package virter

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"text/template"

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
	StorageVolUpload(Vol libvirt.StorageVol, outStream io.Reader, Offset uint64, Length uint64, Flags libvirt.StorageVolUploadFlags) (err error)
}

// Virter manipulates libvirt for virter.
type Virter struct {
	libvirt         LibvirtConnection
	storagePoolName string
	templates       FileReader
}

// New configures a new Virter.
func New(libvirtConnection LibvirtConnection,
	storagePoolName string,
	fileReader FileReader) *Virter {
	return &Virter{
		libvirt:         libvirtConnection,
		storagePoolName: storagePoolName,
		templates:       fileReader,
	}
}

// ImagePull pulls an image from a URL into libvirt.
func (v *Virter) ImagePull(client HTTPClient, url string, name string) error {
	xml, err := v.volumeImageXML(name)
	if err != nil {
		return err
	}

	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get from %v: %w", url, err)
	}
	defer response.Body.Close()

	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	sv, err := v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create storage volume: %w", err)
	}

	err = v.libvirt.StorageVolUpload(sv, response.Body, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to transfer data from URL to libvirt: %w", err)
	}

	fmt.Printf("%v\n", sv.Name)
	return nil
}

func (v *Virter) volumeImageXML(name string) (string, error) {
	templateText, err := v.templates.ReadFile(templateVolumeImage)
	if err != nil {
		return "", fmt.Errorf("could not read template: %w", err)
	}

	t, err := template.New(templateVolumeImage).Parse(string(templateText))
	if err != nil {
		return "", fmt.Errorf("invalid template %v: %w", templateVolumeImage, err)
	}

	templateData := map[string]interface{}{
		"ImageName": name,
	}
	xml := bytes.NewBuffer([]byte{})
	err = t.Execute(xml, templateData)
	if err != nil {
		return "", fmt.Errorf("could not execute template %v: %w", templateVolumeImage, err)
	}

	return xml.String(), nil
}

const templateVolumeImage = "volume-image.xml"
