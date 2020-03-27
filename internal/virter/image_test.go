package virter_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery -name=HTTPClient

func TestImagePull(t *testing.T) {
	client := new(mocks.HTTPClient)
	mockGet(client, http.StatusOK)

	l := newFakeLibvirtConnection()

	v := virter.New(l, poolName, networkName, testDirectory())

	err := v.ImagePull(client, nopReaderProxy{}, imageURL, imageName)
	assert.NoError(t, err)

	client.AssertExpectations(t)

	assert.Len(t, l.vols, 1)
	assert.Equal(t, []byte(imageContent), l.vols[imageName].content)
}

func TestImagePullBadStatus(t *testing.T) {
	client := new(mocks.HTTPClient)
	mockGet(client, http.StatusNotFound)

	l := newFakeLibvirtConnection()

	v := virter.New(l, poolName, networkName, testDirectory())

	err := v.ImagePull(client, nopReaderProxy{}, imageURL, imageName)
	assert.Error(t, err)

	client.AssertExpectations(t)

	assert.Empty(t, l.vols)
}

func mockGet(client *mocks.HTTPClient, statusCode int) {
	response := &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("Status: %v", statusCode),
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(imageContent))),
	}
	client.On("Get", imageURL).Return(response, nil)
}

type nopReaderProxy struct {
}

func (b nopReaderProxy) SetTotal(total int64) {
}

func (b nopReaderProxy) ProxyReader(r io.ReadCloser) io.ReadCloser {
	return r
}

const imageURL = "http://foo.bar"
const imageContent = "some-data"
