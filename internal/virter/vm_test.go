package virter_test

import (
	"fmt"
	"testing"

	"github.com/digitalocean/go-libvirt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery -name=ISOGenerator

func TestVMRun(t *testing.T) {
	directory := prepareVMDirectory()

	g := new(mocks.ISOGenerator)
	mockISOGenerate(g)

	l := new(mocks.LibvirtConnection)
	sp := mockStoragePool(l)

	// ciData
	sv := mockStorageVolCreate(l, sp, ciDataVolumeName, fmt.Sprintf("c0 %v c1", ciDataVolumeName))
	mockStorageVolUpload(l, sv, []byte(ciDataContent))

	// boot volume
	mockBackingVolLookup(l, sp)
	mockStorageVolCreate(l, sp, vmName, fmt.Sprintf("v0 %v v1 %v v2", vmName, backingPath))

	// scratch volume
	mockStorageVolCreate(l, sp, scratchVolumeName, fmt.Sprintf("s0 %v s1", scratchVolumeName))

	// DHCP entry
	n := mockNetworkLookup(l)
	mockNetworkUpdate(
		l, n,
		uint32(libvirt.NetworkUpdateCommandAddLast),
		"<host mac='52:54:00:00:00:2a' ip='192.168.122.42'/>")

	// VM
	d := libvirt.Domain{
		Name: vmName,
	}
	l.On("DomainDefineXML", fmt.Sprintf("d0 %v d1 %v d2", poolName, vmName)).Return(d, nil)
	l.On("DomainCreate", d).Return(nil)

	v := virter.New(l, poolName, networkName, directory)

	c := virter.VMConfig{
		ImageName:    imageName,
		VMName:       vmName,
		VMID:         vmID,
		SSHPublicKey: someSSHKey,
	}
	err := v.VMRun(g, c)
	assert.NoError(t, err)

	l.AssertExpectations(t)
	g.AssertExpectations(t)
}

func prepareVMDirectory() MemoryDirectory {
	directory := MemoryDirectory{}
	directory["volume-cidata.xml"] = []byte("c0 {{.VolumeName}} c1")
	directory["meta-data"] = []byte("meta-data-template")
	directory["user-data"] = []byte("user-data-template")
	directory["volume-vm.xml"] = []byte("v0 {{.VolumeName}} v1 {{.BackingPath}} v2")
	directory["volume-scratch.xml"] = []byte("s0 {{.VolumeName}} s1")
	directory["vm.xml"] = []byte("d0 {{.PoolName}} d1 {{.VMName}} d2")
	return directory
}

func mockISOGenerate(g *mocks.ISOGenerator) {
	g.On("Generate", mock.Anything).Return([]byte(ciDataContent), nil)
}

func mockBackingVolLookup(l *mocks.LibvirtConnection, sp libvirt.StoragePool) {
	sv := mockStorageVolLookup(l, sp, imageName)
	l.On("StorageVolGetPath", sv).Return(backingPath, nil)
}

const (
	ciDataVolume     = "ciDataVolume"
	bootVolume       = "bootVolume"
	scratchVolume    = "scratchVolume"
	domainPersistent = "domainPersistent"
	domainCreated    = "domainCreated"
)

var vmRmTests = []map[string]bool{
	{},
	{
		ciDataVolume: true,
	},
	{
		ciDataVolume: true,
		bootVolume:   true,
	},
	{
		ciDataVolume:  true,
		bootVolume:    true,
		scratchVolume: true,
	},
	{
		ciDataVolume:  true,
		bootVolume:    true,
		scratchVolume: true,
		domainCreated: true,
	},
	{
		ciDataVolume:     true,
		bootVolume:       true,
		scratchVolume:    true,
		domainPersistent: true,
	},
	{
		ciDataVolume:     true,
		bootVolume:       true,
		scratchVolume:    true,
		domainPersistent: true,
		domainCreated:    true,
	},
}

func TestVMRm(t *testing.T) {
	for _, r := range vmRmTests {
		directory := MemoryDirectory{}

		l := new(mocks.LibvirtConnection)
		sp := mockStoragePool(l)

		if r[scratchVolume] {
			sv := mockStorageVolLookup(l, sp, scratchVolumeName)
			mockStorageVolDelete(l, sv)
		} else {
			mockStorageVolNotFound(l, sp, scratchVolumeName)
		}

		if r[bootVolume] {
			sv := mockStorageVolLookup(l, sp, vmName)
			mockStorageVolDelete(l, sv)
		} else {
			mockStorageVolNotFound(l, sp, vmName)
		}

		if r[ciDataVolume] {
			sv := mockStorageVolLookup(l, sp, ciDataVolumeName)
			mockStorageVolDelete(l, sv)
		} else {
			mockStorageVolNotFound(l, sp, ciDataVolumeName)
		}

		if r[domainCreated] || r[domainPersistent] {
			d := mockDomainLookup(l, vmName)
			l.On("DomainGetXMLDesc", d, mock.Anything).Return(domainXML, nil)

			// DHCP entry
			n := mockNetworkLookup(l)
			mockNetworkUpdate(
				l, n,
				uint32(libvirt.NetworkUpdateCommandDelete),
				"<host mac='01:23:45:67:89:ab' ip='192.168.122.2'/>")

			mockDomainActive(l, d, r[domainCreated])
			mockDomainPersistent(l, d, r[domainPersistent])
			mockSnapshotList(l, d)
			if r[domainCreated] {
				mockDomainDestroy(l, d)
			}
			if r[domainPersistent] {
				mockDomainUndefine(l, d)
			}
		} else {
			mockDomainNotFound(l, vmName)
		}

		v := virter.New(l, poolName, networkName, directory)

		err := v.VMRm(vmName)
		assert.NoError(t, err)

		l.AssertExpectations(t)
	}
}

func TestVMCommit(t *testing.T) {
	directory := prepareImageDirectory()

	l := new(mocks.LibvirtConnection)
	sp := mockStoragePool(l)

	sv := mockStorageVolLookup(l, sp, scratchVolumeName)
	mockStorageVolDelete(l, sv)

	sv = mockStorageVolLookup(l, sp, ciDataVolumeName)
	mockStorageVolDelete(l, sv)

	d := mockDomainLookup(l, vmName)
	l.On("DomainGetXMLDesc", d, mock.Anything).Return(domainXML, nil)

	// DHCP entry
	n := mockNetworkLookup(l)
	mockNetworkUpdate(
		l, n,
		uint32(libvirt.NetworkUpdateCommandDelete),
		"<host mac='01:23:45:67:89:ab' ip='192.168.122.2'/>")

	mockDomainActive(l, d, false)
	mockDomainPersistent(l, d, true)
	mockSnapshotList(l, d)
	mockDomainUndefine(l, d)

	v := virter.New(l, poolName, networkName, directory)

	err := v.VMCommit(vmName)
	assert.NoError(t, err)

	l.AssertExpectations(t)
}

func mockStorageVolDelete(l *mocks.LibvirtConnection, sv libvirt.StorageVol) {
	l.On("StorageVolDelete", sv, mock.Anything).Return(nil)
}

func mockStorageVolLookup(l *mocks.LibvirtConnection, sp libvirt.StoragePool, name string) libvirt.StorageVol {
	sv := libvirt.StorageVol{
		Pool: poolName,
		Name: name,
	}
	l.On("StorageVolLookupByName", sp, name).Return(sv, nil)
	return sv
}

func mockStorageVolNotFound(l *mocks.LibvirtConnection, sp libvirt.StoragePool, name string) {
	l.On("StorageVolLookupByName", sp, name).Return(libvirt.StorageVol{}, mockLibvirtError(errNoStorageVol))
}

func mockNetworkLookup(l *mocks.LibvirtConnection) libvirt.Network {
	n := libvirt.Network{
		Name: networkName,
	}
	l.On("NetworkLookupByName", networkName).Return(n, nil)
	l.On("NetworkGetXMLDesc", n, mock.Anything).Return(networkXML, nil)
	return n
}

func mockNetworkUpdate(l *mocks.LibvirtConnection, n libvirt.Network, command uint32, xml string) {
	l.On(
		"NetworkUpdate",
		n,
		uint32(libvirt.NetworkSectionIPDhcpHost),
		command,
		mock.Anything,
		xml,
		mock.Anything).Return(nil)
}

func mockDomainLookup(l *mocks.LibvirtConnection, name string) libvirt.Domain {
	d := libvirt.Domain{
		Name: name,
	}
	l.On("DomainLookupByName", name).Return(d, nil)
	return d
}

func mockDomainActive(l *mocks.LibvirtConnection, d libvirt.Domain, active bool) {
	var rActive int32
	if active {
		rActive = 1
	}
	l.On("DomainIsActive", d).Return(rActive, nil)
}

func mockDomainPersistent(l *mocks.LibvirtConnection, d libvirt.Domain, persistent bool) {
	var rPersistent int32
	if persistent {
		rPersistent = 1
	}
	l.On("DomainIsPersistent", d).Return(rPersistent, nil)
}

func mockSnapshotList(l *mocks.LibvirtConnection, d libvirt.Domain) {
	l.On("DomainListAllSnapshots", d, mock.Anything, mock.Anything).Return([]libvirt.DomainSnapshot{}, int32(0), nil)
}

func mockDomainDestroy(l *mocks.LibvirtConnection, d libvirt.Domain) {
	l.On("DomainDestroy", d).Return(nil)
}

func mockDomainUndefine(l *mocks.LibvirtConnection, d libvirt.Domain) {
	l.On("DomainUndefine", d).Return(nil)
}

func mockDomainNotFound(l *mocks.LibvirtConnection, name string) {
	l.On("DomainLookupByName", name).Return(libvirt.Domain{}, mockLibvirtError(errNoDomain))
}

func mockLibvirtError(code errorNumber) error {
	return libvirtError{uint32(code)}
}

type libvirtError struct {
	Code uint32
}

func (e libvirtError) Error() string {
	return fmt.Sprintf("libvirt error code %v", e.Code)
}

type errorNumber int32

const (
	errNoDomain     errorNumber = 42
	errNoStorageVol errorNumber = 50
)

const (
	vmName            = "some-vm"
	vmID              = 42
	ciDataVolumeName  = vmName + "-cidata"
	scratchVolumeName = vmName + "-scratch"
	ciDataContent     = "some-ci-data"
	backingPath       = "/some/path"
	someSSHKey        = "some-key"
)

const networkXML = `<network>
  <ip address='192.168.122.1' netmask='255.255.255.0'>
    <dhcp>
      <host mac='01:23:45:67:89:ab' ip='192.168.122.2'/>
    </dhcp>
  </ip>
</network>
`

const domainXML = `<domain>
  <devices>
    <interface type='network'>
      <mac address='01:23:45:67:89:ab'/>
    </interface>
  </devices>
</domain>
`
