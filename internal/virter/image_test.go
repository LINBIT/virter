package virter_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/digitalocean/go-libvirt"
	"github.com/stretchr/testify/assert"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery -name=HTTPClient

func TestImagePull(t *testing.T) {
	directory := prepareImageDirectory()

	client := new(mocks.HTTPClient)
	mockGet(client, http.StatusOK)

	l := new(mocks.LibvirtConnection)
	sp := mockStoragePool(l)
	sv := mockImageVolCreate(l, sp)
	mockStorageVolUpload(l, sv, []byte(imageContent))

	v := virter.New(l, poolName, directory)

	err := v.ImagePull(client, nopReaderProxy{}, imageURL, imageName)
	assert.NoError(t, err)

	client.AssertExpectations(t)
	l.AssertExpectations(t)
}

func TestImagePullBadStatus(t *testing.T) {
	directory := prepareImageDirectory()

	client := new(mocks.HTTPClient)
	mockGet(client, http.StatusNotFound)

	l := new(mocks.LibvirtConnection)
	// mock libvirt interactions because ImagePull is free to create the
	// volume before performing the HTTP GET
	sp := mockStoragePool(l)
	_ = mockImageVolCreate(l, sp)

	v := virter.New(l, poolName, directory)

	err := v.ImagePull(client, nopReaderProxy{}, imageURL, imageName)
	assert.Error(t, err)

	client.AssertExpectations(t)
}

func prepareImageDirectory() MemoryDirectory {
	directory := MemoryDirectory{}
	directory["volume-image.xml"] = []byte("t0 {{.ImageName}} t1")
	return directory
}

func mockGet(client *mocks.HTTPClient, statusCode int) {
	response := &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("Status: %v", statusCode),
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(imageContent))),
	}
	client.On("Get", imageURL).Return(response, nil)
}

func mockImageVolCreate(l *mocks.LibvirtConnection, sp libvirt.StoragePool) libvirt.StorageVol {
	return mockStorageVolCreate(l, sp, imageName, fmt.Sprintf("t0 %v t1", imageName))
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
