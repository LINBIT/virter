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

	err := v.VMRun(g, imageName, vmName)
	assert.NoError(t, err)

	l.AssertExpectations(t)
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
	sv := libvirt.StorageVol{
		Pool: poolName,
		Name: imageName,
	}
	l.On("StorageVolLookupByName", sp, imageName).Return(sv, nil)
	l.On("StorageVolGetPath", sv).Return(backingPath, nil)
}

const vmName = "some-vm"
const ciDataVolumeName = vmName + "-cidata"
const scratchVolumeName = vmName + "-scratch"
const ciDataContent = "some-ci-data"
const backingPath = "/some/path"
