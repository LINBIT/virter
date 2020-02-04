package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/LINBIT/virter/internal"
	"github.com/LINBIT/virter/internal/mocks"
)

//go:generate mockery -name=HTTPClient
//go:generate mockery -name=LibvirtConnect
//go:generate mockery -name=LibvirtStoragePool
//go:generate mockery -name=LibvirtStream

func TestPull(t *testing.T) {
	mock := new(mocks.HTTPClient)
	mock.On("Get", "http://foo.bar").Return(nil, nil)

	sp := new(mocks.LibvirtStoragePool)
	sp.On("GetUUIDString").Return("some-uuid", nil)

	conn := new(mocks.LibvirtConnect)
	conn.On("LookupStoragePoolByName", "images").Return(sp, nil)

	err := ImagePull(conn, mock, "http://foo.bar")
	assert.Nil(t, err)
	mock.AssertExpectations(t)
}
