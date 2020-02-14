package virter

import (
	"bytes"
	"fmt"
	"io"
	"text/template"

	"github.com/digitalocean/go-libvirt"
)

// FileReader is the interface for reading whole files.
type FileReader interface {
	ReadFile(subpath string) ([]byte, error)
}

// LibvirtConnection contains required libvirt connection methods.
type LibvirtConnection interface {
	StoragePoolLookupByName(Name string) (rPool libvirt.StoragePool, err error)
	StorageVolLookupByName(Pool libvirt.StoragePool, Name string) (rVol libvirt.StorageVol, err error)
	StorageVolCreateXML(Pool libvirt.StoragePool, XML string, Flags libvirt.StorageVolCreateFlags) (rVol libvirt.StorageVol, err error)
	StorageVolUpload(Vol libvirt.StorageVol, outStream io.Reader, Offset uint64, Length uint64, Flags libvirt.StorageVolUploadFlags) (err error)
	StorageVolGetPath(Vol libvirt.StorageVol) (rName string, err error)
	DomainDefineXML(XML string) (rDom libvirt.Domain, err error)
	DomainCreate(Dom libvirt.Domain) (err error)
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

func (v *Virter) renderTemplate(name string, data interface{}) (string, error) {
	templateText, err := v.templates.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("could not read template: %w", err)
	}

	t, err := template.New(name).Parse(string(templateText))
	if err != nil {
		return "", fmt.Errorf("invalid template %v: %w", name, err)
	}

	xml := bytes.NewBuffer([]byte{})
	err = t.Execute(xml, data)
	if err != nil {
		return "", fmt.Errorf("could not execute template %v: %w", name, err)
	}

	return xml.String(), nil
}
