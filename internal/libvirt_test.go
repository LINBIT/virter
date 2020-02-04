package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LINBIT/virter/internal/mocks"
)

//go:generate mockery -name=HTTPClient

func TestPull(t *testing.T) {
	mock := new(mocks.HTTPClient)
	mock.On("Get", "http://foo.bar").Return(nil, nil)
	err := ImagePull(mock, "http://foo.bar")
	assert.Nil(t, err)
	mock.AssertExpectations(t)
}
