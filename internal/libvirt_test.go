package internal_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/digitalocean/go-libvirt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	. "github.com/LINBIT/virter/internal"
	"github.com/LINBIT/virter/internal/mocks"
)

//go:generate mockery -name=HTTPClient
//go:generate mockery -name=LibvirtConn

func TestPull(t *testing.T) {
	directory := MemoryDirectory{}
	directory["volume-image.xml"] = []byte("some-xml")

	client := new(mocks.HTTPClient)

	resp := &http.Response{
		Body: ioutil.NopCloser(bytes.NewReader([]byte("some-content"))),
	}
	client.On("Get", "http://foo.bar").Return(resp, nil)

	conn := new(mocks.LibvirtConn)

	sp := libvirt.StoragePool{
		Name: poolName,
	}
	conn.On("StoragePoolLookupByName", poolName).Return(sp, nil)

	sv := libvirt.StorageVol{
		Pool: poolName,
		Name: volName,
	}
	conn.On("StorageVolCreateXML", sp, mock.Anything, mock.Anything).Return(sv, nil)

	v := New(conn, directory)

	err := v.ImagePull(client, "http://foo.bar")
	assert.NoError(t, err)

	client.AssertExpectations(t)
	conn.AssertExpectations(t)
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
