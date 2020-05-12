package virter

import (
	"bytes"
	"fmt"
	"io"
	"text/template"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
	"golang.org/x/crypto/ssh"
)

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
	DomainShutdown(Dom libvirt.Domain) (err error)
	DomainDestroy(Dom libvirt.Domain) (err error)
	DomainUndefine(Dom libvirt.Domain) (err error)
	DomainListAllSnapshots(Dom libvirt.Domain, NeedResults int32, Flags uint32) (rSnapshots []libvirt.DomainSnapshot, rRet int32, err error)
	DomainSnapshotDelete(Snap libvirt.DomainSnapshot, Flags libvirt.DomainSnapshotDeleteFlags) (err error)
	LifecycleEvents() (<-chan libvirt.DomainEventLifecycleMsg, error)
	Disconnect() error
}

// Virter manipulates libvirt for virter.
type Virter struct {
	libvirt         LibvirtConnection
	storagePoolName string
	networkName     string
}

// New configures a new Virter.
func New(libvirtConnection LibvirtConnection,
	storagePoolName string,
	networkName string) *Virter {
	return &Virter{
		libvirt:         libvirtConnection,
		storagePoolName: storagePoolName,
		networkName:     networkName,
	}
}

// Disconnect disconnects virter's connection to libvirt
func (v *Virter) Disconnect() error {
	return v.libvirt.Disconnect()
}

// VMConfig contains the configuration for starting a VM
type VMConfig struct {
	ImageName     string
	Name          string
	MemoryKiB     uint64
	VCPUs         uint
	ID            uint
	SSHPublicKeys []string
	SSHPrivateKey []byte
	WaitSSH       bool
	SSHPingCount  int
	SSHPingPeriod time.Duration
	ConsoleFile   *VMConsoleFile
}

type VMConsoleFile struct {
	Path     string
	OwnerUID uint32
	OwnerGID uint32
}

// CheckVMConfig takes a VMConfig, does basic checks, and returns it back.
func CheckVMConfig(vmConfig VMConfig) (VMConfig, error) {
	// I don't want to put any arbitrary limits on the amount of mem,
	// but this protects against the zero value case
	if vmConfig.MemoryKiB == 0 {
		return vmConfig, fmt.Errorf("cannot start a VM with 0 memory")
	} else if vmConfig.VCPUs == 0 {
		return vmConfig, fmt.Errorf("cannot start a VM with 0 (virtual) CPUs")
	} else if vmConfig.ID == 1 {
		return vmConfig, fmt.Errorf("cannot start a VM with reserved ID (i.e., IP) 'x.y.z.%d'", vmConfig.ID)
	}

	return vmConfig, nil
}

// ShellClientBuilder provides SSH connections
type ShellClientBuilder interface {
	NewShellClient(hostPort string, sshconfig ssh.ClientConfig) ShellClient
}

// ShellClient executes shell commands
type ShellClient interface {
	Dial() error
	Close() error
	StdoutPipe() (io.Reader, error)
	StderrPipe() (io.Reader, error)
	ExecScript(script string) error
	Shell() error
}

// AfterNotifier wait for a duration to elapse
type AfterNotifier interface {
	After(d time.Duration) <-chan time.Time
}

// NetworkCopier copies files over the network
type NetworkCopier interface {
	// Copy transfers a list of files (source) from the local machine to
	// a target directory (destination) on a remote host (host).
	Copy(host string, source []string, destination string) error
}

func renderTemplate(name, content string, data interface{}) (string, error) {
	t, err := template.New(name).Parse(content)
	if err != nil {
		return "", fmt.Errorf("invalid template %v: %w", name, err)
	}

	result := bytes.NewBuffer([]byte{})
	err = t.Execute(result, data)
	if err != nil {
		return "", fmt.Errorf("could not execute template %v: %w", name, err)
	}

	return result.String(), nil
}
