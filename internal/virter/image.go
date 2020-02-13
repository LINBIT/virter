package virter

import (
	"fmt"
	"io"
	"net/http"
)

// HTTPClient contains required HTTP methods.
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// ReaderProxy wraps reading from a Reader with a known total size.
type ReaderProxy interface {
	SetTotal(total int64)
	ProxyReader(r io.ReadCloser) io.ReadCloser
}

// ImagePull pulls an image from a URL into libvirt.
func (v *Virter) ImagePull(client HTTPClient, readerProxy ReaderProxy, url string, name string) error {
	xml, err := v.volumeImageXML(name)
	if err != nil {
		return err
	}

	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get from %v: %w", url, err)
	}
	readerProxy.SetTotal(response.ContentLength)
	proxyResponse := readerProxy.ProxyReader(response.Body)
	defer proxyResponse.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("error %v from %v", response.Status, url)
	}

	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	sv, err := v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create storage volume: %w", err)
	}

	err = v.libvirt.StorageVolUpload(sv, proxyResponse, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to transfer data from URL to libvirt: %w", err)
	}

	return nil
}

func (v *Virter) volumeImageXML(name string) (string, error) {
	templateData := map[string]interface{}{
		"ImageName": name,
	}

	return v.renderTemplate(templateVolumeImage, templateData)
}

const templateVolumeImage = "volume-image.xml"
