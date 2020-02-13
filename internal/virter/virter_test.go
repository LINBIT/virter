package virter_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/digitalocean/go-libvirt"
	"github.com/stretchr/testify/mock"

	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery -name=LibvirtConnection

func mockStoragePool(l *mocks.LibvirtConnection) libvirt.StoragePool {
	sp := libvirt.StoragePool{
		Name: poolName,
	}
	l.On("StoragePoolLookupByName", poolName).Return(sp, nil)
	return sp
}

func mockStorageVolCreate(l *mocks.LibvirtConnection, sp libvirt.StoragePool, name string, expectedXML string) libvirt.StorageVol {
	sv := libvirt.StorageVol{
		Pool: poolName,
		Name: name,
	}
	l.On("StorageVolCreateXML", sp, expectedXML, mock.Anything).Return(sv, nil)
	return sv
}

func mockStorageVolUpload(l *mocks.LibvirtConnection, sv libvirt.StorageVol, content []byte) {
	l.On("StorageVolUpload",
		sv,
		readerMatcher(content),
		uint64(0),
		uint64(0),
		mock.Anything).Return(nil)
}

func readerMatcher(expected []byte) interface{} {
	return mock.MatchedBy(func(r io.Reader) bool {
		data, err := ioutil.ReadAll(r)
		if err != nil {
			panic("error reading data to test match")
		}
		return bytes.Equal(data, expected)
	})
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
const imageName = "some-image"
