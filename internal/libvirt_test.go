package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	libvirtmocks "github.com/LINBIT/virter/internal/libvirtinterfaces/mocks"
	"github.com/LINBIT/virter/internal/mocks"
)

//go:generate mockery -name=HTTPClient

func TestPull(t *testing.T) {
	mock := new(mocks.HTTPClient)
	mock.On("Get", "http://foo.bar").Return(nil, nil)

	sp := new(libvirtmocks.LibvirtStoragePool)
	sp.On("GetUUIDString").Return("some-uuid", nil)

	conn := new(libvirtmocks.LibvirtConnect)
	conn.On("LookupStoragePoolByName", "images").Return(sp, nil)

	err := ImagePull(conn, mock, "http://foo.bar")
	assert.Nil(t, err)
	mock.AssertExpectations(t)
}
