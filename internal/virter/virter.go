package virter

import (
	"bytes"
	"fmt"
	"io"
	"net"
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
	StorageVolCreateXML(Pool libvirt.StoragePool, XML string, Flags libvirt.StorageVolCreateFlags) (rVol libvirt.StorageVol, err error)
	StorageVolDelete(Vol libvirt.StorageVol, Flags libvirt.StorageVolDeleteFlags) (err error)
	StorageVolGetPath(Vol libvirt.StorageVol) (rName string, err error)
	StorageVolLookupByName(Pool libvirt.StoragePool, Name string) (rVol libvirt.StorageVol, err error)
	StorageVolUpload(Vol libvirt.StorageVol, outStream io.Reader, Offset uint64, Length uint64, Flags libvirt.StorageVolUploadFlags) (err error)
	NetworkLookupByName(Name string) (rNet libvirt.Network, err error)
	NetworkGetXMLDesc(Net libvirt.Network, Flags uint32) (rXML string, err error)
	NetworkUpdate(Net libvirt.Network, Command uint32, Section uint32, ParentIndex int32, XML string, Flags libvirt.NetworkUpdateFlags) (err error)
	DomainLookupByName(Name string) (rDom libvirt.Domain, err error)
	DomainGetXMLDesc(Dom libvirt.Domain, Flags libvirt.DomainXMLFlags) (rXML string, err error)
	DomainDefineXML(XML string) (rDom libvirt.Domain, err error)
	DomainCreate(Dom libvirt.Domain) (err error)
	DomainIsActive(Dom libvirt.Domain) (rActive int32, err error)
	DomainIsPersistent(Dom libvirt.Domain) (rPersistent int32, err error)
	DomainDestroy(Dom libvirt.Domain) (err error)
	DomainUndefine(Dom libvirt.Domain) (err error)
	DomainListAllSnapshots(Dom libvirt.Domain, NeedResults int32, Flags uint32) (rSnapshots []libvirt.DomainSnapshot, rRet int32, err error)
	DomainSnapshotDelete(Snap libvirt.DomainSnapshot, Flags libvirt.DomainSnapshotDeleteFlags) (err error)
}

// Virter manipulates libvirt for virter.
type Virter struct {
	libvirt         LibvirtConnection
	storagePoolName string
	networkName     string
	templates       FileReader
}

// New configures a new Virter.
func New(libvirtConnection LibvirtConnection,
	storagePoolName string,
	networkName string,
	fileReader FileReader) *Virter {
	return &Virter{
		libvirt:         libvirtConnection,
		storagePoolName: storagePoolName,
		networkName:     networkName,
		templates:       fileReader,
	}
}

// VMConfig contains the configuration for starting a VM
type VMConfig struct {
	ImageName    string
	VMName       string
	VMID         uint
	SSHPublicKey string
}

// ISOGenerator generates ISO images from file data
type ISOGenerator interface {
	Generate(files map[string][]byte) ([]byte, error)
}

// PortWaiter waits for TCP ports to be open
type PortWaiter interface {
	WaitPort(ip net.IP, port string) error
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
