package virter

import (
	"io"

	"github.com/digitalocean/go-libvirt"
)

// FileReader is the interface for reading whole files.
type FileReader interface {
	ReadFile(subpath string) ([]byte, error)
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
