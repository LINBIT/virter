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

	// VM
	d := libvirt.Domain{
		Name: vmName,
	}
	l.On("DomainDefineXML", fmt.Sprintf("d0 %v d1 %v d2", poolName, vmName)).Return(d, nil)
	l.On("DomainCreate", d).Return(nil)

	v := virter.New(l, poolName, directory)

	err := v.VMRun(g, imageName, vmName, someSSHKey)
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
	ciDataVolume  = "ciDataVolume"
	bootVolume    = "bootVolume"
	scratchVolume = "scratchVolume"
	domainDefined = "domainDefined"
	domainCreated = "domainCreated"
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

		v := virter.New(l, poolName, directory)

		err := v.VMRm(vmName)
		assert.NoError(t, err)

		l.AssertExpectations(t)
	}
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
	errNoStorageVol errorNumber = 50
)

const (
	vmName            = "some-vm"
	ciDataVolumeName  = vmName + "-cidata"
	scratchVolumeName = vmName + "-scratch"
	ciDataContent     = "some-ci-data"
	backingPath       = "/some/path"
	someSSHKey        = "some-key"
)
