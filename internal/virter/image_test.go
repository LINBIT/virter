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
	"github.com/stretchr/testify/mock"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery -name=HTTPClient

func TestPull(t *testing.T) {
	directory := prepareDirectory()

	client := new(mocks.HTTPClient)
	mockGet(client, http.StatusOK)

	l := new(mocks.LibvirtConnection)
	sp := mockStoragePool(l)
	sv := mockStorageVolCreate(l, sp)
	mockStorageVolUpload(l, sv)

	v := virter.New(l, poolName, directory)

	err := v.ImagePull(client, nopReaderProxy{}, imageURL, volName)
	assert.NoError(t, err)

	client.AssertExpectations(t)
	l.AssertExpectations(t)
}

func TestPullBadStatus(t *testing.T) {
	directory := prepareDirectory()

	client := new(mocks.HTTPClient)
	mockGet(client, http.StatusNotFound)

	l := new(mocks.LibvirtConnection)
	sp := mockStoragePool(l)
	sv := mockStorageVolCreate(l, sp)
	mockStorageVolUpload(l, sv)

	v := virter.New(l, poolName, directory)

	err := v.ImagePull(client, nopReaderProxy{}, imageURL, volName)
	assert.Error(t, err)

	client.AssertExpectations(t)
}

func mockGet(client *mocks.HTTPClient, statusCode int) {
	response := &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("Status: %v", statusCode),
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(imageContent))),
	}
	client.On("Get", imageURL).Return(response, nil)
}

func mockStorageVolUpload(l *mocks.LibvirtConnection, sv libvirt.StorageVol) {
	l.On("StorageVolUpload",
		sv,
		readerMatcher([]byte(imageContent)),
		uint64(0),
		uint64(0),
		mock.Anything).Return(nil)
}

type nopReaderProxy struct {
}

func (b nopReaderProxy) SetTotal(total int64) {
}

func (b nopReaderProxy) ProxyReader(r io.ReadCloser) io.ReadCloser {
	return r
}

func readerMatcher(expected []byte) interface{} {
	return mock.MatchedBy(func(r io.Reader) bool {
		data, err := ioutil.ReadAll(r)
		if err != nil {
			panic("error reading data to test match")
		}
		return bytes.Equal(data, expected)
	})
}

const imageURL = "http://foo.bar"
const imageContent = "some-data"
