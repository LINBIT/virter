package virter_test

import (
	"fmt"

	"github.com/digitalocean/go-libvirt"
	"github.com/stretchr/testify/mock"

	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery -name=LibvirtConnection

func prepareDirectory() MemoryDirectory {
	directory := MemoryDirectory{}
	directory["volume-image.xml"] = []byte("t0 {{.ImageName}} t1")
	return directory
}

func mockStoragePool(l *mocks.LibvirtConnection) libvirt.StoragePool {
	sp := libvirt.StoragePool{
		Name: poolName,
	}
	l.On("StoragePoolLookupByName", poolName).Return(sp, nil)
	return sp
}

func mockStorageVolCreate(l *mocks.LibvirtConnection, sp libvirt.StoragePool) libvirt.StorageVol {
	sv := libvirt.StorageVol{
		Pool: poolName,
		Name: volName,
	}
	xml := fmt.Sprintf("t0 %v t1", volName)
	l.On("StorageVolCreateXML", sp, xml, mock.Anything).Return(sv, nil)
	return sv
}

type MemoryDirectory map[string][]byte

func (d MemoryDirectory) ReadFile(subpath string) ([]byte, error) {
	v, ok := d[subpath]
	if !ok {
		return nil, fmt.Errorf("no file found at %v", subpath)
	}
	return v, nil
}

const poolName = "some-pool"
const volName = "vol"
