package internal_test

import (
	"testing"

	"github.com/digitalocean/go-libvirt"
	"github.com/stretchr/testify/assert"

	. "github.com/LINBIT/virter/internal"
	"github.com/LINBIT/virter/internal/mocks"
)

//go:generate mockery -name=HTTPClient
//go:generate mockery -name=LibvirtConn

func TestPull(t *testing.T) {
	mock := new(mocks.HTTPClient)
	mock.On("Get", "http://foo.bar").Return(nil, nil)

	var dummyUUID [libvirt.VirUUIDBuflen]byte
	sp := libvirt.StoragePool{
		Name: "images",
		UUID: dummyUUID,
	}

	conn := new(mocks.LibvirtConn)
	conn.On("StoragePoolLookupByName", "images").Return(sp, nil)

	err := ImagePull(conn, mock, "http://foo.bar")
	assert.Nil(t, err)
	mock.AssertExpectations(t)
}
