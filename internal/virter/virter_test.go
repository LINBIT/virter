package virter_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/digitalocean/go-libvirt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	. "github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery -name=HTTPClient
//go:generate mockery -name=LibvirtConnection

func TestPull(t *testing.T) {
	directory := MemoryDirectory{}
	directory["volume-image.xml"] = []byte("t0 {{.ImageName}} t1")

	client := new(mocks.HTTPClient)

	response := &http.Response{
		Body: ioutil.NopCloser(bytes.NewReader([]byte("some-content"))),
	}
	client.On("Get", "http://foo.bar").Return(response, nil)

	l := new(mocks.LibvirtConnection)

	sp := libvirt.StoragePool{
		Name: poolName,
	}
	l.On("StoragePoolLookupByName", poolName).Return(sp, nil)

	sv := libvirt.StorageVol{
		Pool: poolName,
		Name: volName,
	}
	l.On("StorageVolCreateXML", sp, "t0 some-name t1", mock.Anything).Return(sv, nil)

	v := New(l, directory)

	err := v.ImagePull(client, "http://foo.bar")
	assert.NoError(t, err)

	client.AssertExpectations(t)
	l.AssertExpectations(t)
}

type MemoryDirectory map[string][]byte

func (d MemoryDirectory) ReadFile(subpath string) ([]byte, error) {
	v, ok := d[subpath]
	if !ok {
		return nil, fmt.Errorf("no file found at %v", subpath)
	}
	return v, nil
}

const poolName = "images"
const volName = "vol"
