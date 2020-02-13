package virter_test

import (
	"fmt"
	"testing"

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
	sv := mockStorageVolCreate(l, sp, ciDataVolName, fmt.Sprintf("c0 %v c1", ciDataVolName))
	mockStorageVolUpload(l, sv, []byte(ciDataContent))

	v := virter.New(l, poolName, directory)

	err := v.VMRun(g, vmName)
	assert.NoError(t, err)

	l.AssertExpectations(t)
}

func prepareVMDirectory() MemoryDirectory {
	directory := MemoryDirectory{}
	directory["volume-cidata.xml"] = []byte("c0 {{.VolumeName}} c1")
	directory["meta-data"] = []byte("meta-data-template")
	directory["user-data"] = []byte("user-data-template")
	return directory
}

func mockISOGenerate(g *mocks.ISOGenerator) {
	g.On("Generate", mock.Anything).Return([]byte(ciDataContent), nil)
}

const vmName = "some-vm"
const ciDataVolName = vmName + "-cidata"
const ciDataContent = "some-ci-data"
