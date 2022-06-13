package virter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"text/template"
	"time"

	"github.com/digitalocean/go-libvirt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/LINBIT/virter/pkg/sshkeys"
)

// LibvirtConnection contains required libvirt connection methods.
type LibvirtConnection interface {
	ConnectListAllDomains(NeedResults int32, Flags libvirt.ConnectListAllDomainsFlags) (rDomains []libvirt.Domain, rRet uint32, err error)
	StoragePoolLookupByName(Name string) (rPool libvirt.StoragePool, err error)
	StoragePoolListAllVolumes(Pool libvirt.StoragePool, NeedResults int32, Flags uint32) (rVols []libvirt.StorageVol, rRet uint32, err error)
	StorageVolCreateXML(Pool libvirt.StoragePool, XML string, Flags libvirt.StorageVolCreateFlags) (rVol libvirt.StorageVol, err error)
	StorageVolCreateXMLFrom(Pool libvirt.StoragePool, XML string, original libvirt.StorageVol, Flags libvirt.StorageVolCreateFlags) (rVol libvirt.StorageVol, err error)
	StorageVolDelete(Vol libvirt.StorageVol, Flags libvirt.StorageVolDeleteFlags) (err error)
	StorageVolLookupByName(Pool libvirt.StoragePool, Name string) (rVol libvirt.StorageVol, err error)
	StorageVolUpload(Vol libvirt.StorageVol, outStream io.Reader, Offset, Length uint64, Flags libvirt.StorageVolUploadFlags) (err error)
	StorageVolGetXMLDesc(Vol libvirt.StorageVol, Flags uint32) (rXML string, err error)
	StorageVolDownload(Vol libvirt.StorageVol, inStream io.Writer, Offset, Length uint64, Flags libvirt.StorageVolDownloadFlags) (err error)
	StorageVolGetInfo(Vol libvirt.StorageVol) (rType int8, rCapacity, rAllocation uint64, err error)
	ConnectListAllNetworks(NeedResults int32, Flags libvirt.ConnectListAllNetworksFlags) (rNets []libvirt.Network, rRet uint32, err error)
	NetworkGetDhcpLeases(Net libvirt.Network, Mac libvirt.OptString, NeedResults int32, Flags uint32) (rLeases []libvirt.NetworkDhcpLease, rRet uint32, err error)
	NetworkDefineXML(XML string) (rNet libvirt.Network, err error)
	NetworkSetAutostart(Net libvirt.Network, Autostart int32) (err error)
	NetworkCreate(Net libvirt.Network) (err error)
	NetworkDestroy(Net libvirt.Network) (err error)
	NetworkUndefine(Net libvirt.Network) (err error)
	NetworkLookupByName(Name string) (rNet libvirt.Network, err error)
	NetworkGetXMLDesc(Net libvirt.Network, Flags uint32) (rXML string, err error)
	NetworkUpdate(Net libvirt.Network, Command, Section uint32, ParentIndex int32, XML string, Flags libvirt.NetworkUpdateFlags) (err error)
	DomainLookupByName(Name string) (rDom libvirt.Domain, err error)
	DomainGetXMLDesc(Dom libvirt.Domain, Flags libvirt.DomainXMLFlags) (rXML string, err error)
	DomainDefineXML(XML string) (rDom libvirt.Domain, err error)
	DomainCreate(Dom libvirt.Domain) (err error)
	DomainIsActive(Dom libvirt.Domain) (rActive int32, err error)
	DomainIsPersistent(Dom libvirt.Domain) (rPersistent int32, err error)
	DomainShutdown(Dom libvirt.Domain) (err error)
	DomainDestroy(Dom libvirt.Domain) (err error)
	DomainUndefineFlags(Dom libvirt.Domain, Flags libvirt.DomainUndefineFlagsValues) (err error)
	DomainListAllSnapshots(Dom libvirt.Domain, NeedResults int32, Flags uint32) (rSnapshots []libvirt.DomainSnapshot, rRet int32, err error)
	DomainSnapshotDelete(Snap libvirt.DomainSnapshot, Flags libvirt.DomainSnapshotDeleteFlags) (err error)
	Disconnect() error
	ConnectSupportsFeature(Feature int32) (int32, error)
	ConnectGetDomainCapabilities(Emulatorbin libvirt.OptString, Arch libvirt.OptString, Machine libvirt.OptString, Virttype libvirt.OptString, Flags uint32) (rCapabilities string, err error)
}

// Virter manipulates libvirt for virter.
type Virter struct {
	libvirt              LibvirtConnection
	provisionStoragePool libvirt.StoragePool
	provisionNetwork     libvirt.Network
	sshkeys              sshkeys.KeyStore
}

// New configures a new Virter.
func New(libvirtConnection LibvirtConnection,
	storagePoolName string,
	networkName string,
	store sshkeys.KeyStore) *Virter {
	// We intentionally allow these to be null. Not every command requires them and for example the network commands
	// could be used to bootstrap the actual virter network.
	sp, err := libvirtConnection.StoragePoolLookupByName(storagePoolName)
	if err != nil {
		log.WithError(err).Warnf("could not look up storage pool %s", storagePoolName)
	}
	net, err := libvirtConnection.NetworkLookupByName(networkName)
	if err != nil {
		log.WithError(err).Warnf("could not look up network %s", networkName)
	}

	return &Virter{
		libvirt:              libvirtConnection,
		provisionStoragePool: sp,
		provisionNetwork:     net,
		sshkeys:              store,
	}
}

// Disconnect disconnects virter's connection to libvirt
func (v *Virter) Disconnect() error {
	return v.libvirt.Disconnect()
}

// ForceDisconnect disconnects virter's connection to libvirt
//
// It behaves like Disconnect(), except it does not return an error.
// If an error would be returned, the error will be logged and the program will terminate.
// Note: this is useful for `defer` statements
func (v *Virter) ForceDisconnect() {
	err := v.Disconnect()
	if err != nil {
		log.WithError(err).Fatalf("failed to disconnect from libvirt")
	}
}

type Disk interface {
	GetName() string
	GetSizeKiB() uint64
	GetFormat() string
	GetBus() string
}

type NICType string

const (
	NICTypeNetwork = "network"
	NICTypeBridge  = "bridge"
)

type NIC interface {
	GetType() string
	GetSource() string
	GetModel() string
	GetMAC() string
}

type Mount interface {
	GetHostPath() string
	GetVMPath() string
}

// VMConfig contains the configuration for starting a VM
type VMConfig struct {
	Image              *LocalImage
	CpuArch            CpuArch
	Name               string
	MemoryKiB          uint64
	BootCapacityKiB    uint64
	VCPUs              uint
	ID                 uint
	StaticDHCP         bool
	ExtraSSHPublicKeys []string
	ConsolePath        string
	Disks              []Disk
	DiskCache          string
	ExtraNics          []NIC
	Mounts             []Mount
	GDBPort            uint
	SecureBoot         bool
	VNCEnabled	   bool
	VNCPort            int
}

// VMMeta is additional metadata stored with each VM
type VMMeta struct {
	HostKey string `xml:"hostkey"`
}

// VmReadyConfig contains the configuration for waiting for a VM to be ready.
type VmReadyConfig struct {
	Retries      int
	CheckTimeout time.Duration
}

func checkDisks(vmConfig VMConfig) error {
	for _, d := range vmConfig.Disks {
		if _, ok := busToDevPrefix[d.GetBus()]; !ok {
			return fmt.Errorf("cannot attach disk '%s' with unknown bus type '%s'", d.GetName(), d.GetBus())
		}
		if !approvedDiskFormats[d.GetFormat()] {
			return fmt.Errorf("cannot attach disk '%s' with unknown format '%s'", d.GetName(), d.GetFormat())
		}
	}

	return nil
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
	} else if err := checkDisks(vmConfig); err != nil {
		return vmConfig, fmt.Errorf("cannot start VM: %w", err)
	} else if vmConfig.VNCEnabled && (vmConfig.VNCPort < 5900 || vmConfig.VNCPort > 65535) {
		return vmConfig, fmt.Errorf("VNC port must be in the range [5900 65535]: port is %v", vmConfig.VNCPort)
	}

	return vmConfig, nil
}

// ShellClientBuilder provides SSH connections
type ShellClientBuilder interface {
	NewShellClient(hostPort string, sshconfig ssh.ClientConfig) ShellClient
}

// ShellClient executes shell commands
type ShellClient interface {
	DialContext(ctx context.Context) error
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
