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

	. "github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery -name=HTTPClient
//go:generate mockery -name=LibvirtConnection

func TestPull(t *testing.T) {
	directory := prepareDirectory()

	client := new(mocks.HTTPClient)
	mockGet(client, http.StatusOK)

	l := new(mocks.LibvirtConnection)
	sp := mockStoragePool(l)
	sv := mockStorageVolCreate(l, sp)
	mockStorageVolUpload(l, sv)

	v := New(l, poolName, directory)

	err := v.ImagePull(client, imageURL, volName)
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

	v := New(l, poolName, directory)

	err := v.ImagePull(client, imageURL, volName)
	assert.Error(t, err)

	client.AssertExpectations(t)
}

func prepareDirectory() MemoryDirectory {
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

func mockStoragePool(l *mocks.LibvirtConnection) libvirt.StoragePool {
	sp := libvirt.StoragePool{
		Name: poolName,
	}
	l.On("StoragePoolLookupByName", poolName).Return(sp, nil)
	return sp
}

func mockStorageVolCreate(l *mocks.LibvirtConnection, sp libvirt.StoragePool) libvirt.StorageVol {
	sv := libvirt.StorageVol{
		Pool: poolName,
		Name: volName,
	}
	xml := fmt.Sprintf("t0 %v t1", volName)
	l.On("StorageVolCreateXML", sp, xml, mock.Anything).Return(sv, nil)
	return sv
}

func mockStorageVolUpload(l *mocks.LibvirtConnection, sv libvirt.StorageVol) {
	l.On("StorageVolUpload",
		sv,
		readerMatcher([]byte(imageContent)),
		uint64(0),
		uint64(0),
		mock.Anything).Return(nil)
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

type MemoryDirectory map[string][]byte

func (d MemoryDirectory) ReadFile(subpath string) ([]byte, error) {
	v, ok := d[subpath]
	if !ok {
		return nil, fmt.Errorf("no file found at %v", subpath)
	}
	return v, nil
}

const poolName = "some-pool"
const volName = "vol"
const imageURL = "http://foo.bar"
const imageContent = "some-data"
