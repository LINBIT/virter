package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LINBIT/virter/internal/mocks"
)

//go:generate mockery -name=HttpClient

func TestPull(t *testing.T) {
	mock := new(mocks.HttpClient)
	mock.On("Get", "http://foo.bar").Return(nil, nil)
	err := ImagePull(mock, "http://foo.bar")
	assert.Nil(t, err)
	mock.AssertExpectations(t)
}
