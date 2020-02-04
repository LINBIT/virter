package internal_test

import (
	"bytes"
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

	v := New(conn, "")

	err := v.ImagePull(client, "http://foo.bar")
	assert.NoError(t, err)

	client.AssertExpectations(t)
	conn.AssertExpectations(t)
}

const poolName = "images"
const volName = "vol"
