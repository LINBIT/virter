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
	directory := MemoryDirectory{}
	directory["volume-image.xml"] = []byte("t0 {{.ImageName}} t1")

	client := new(mocks.HTTPClient)

	response := &http.Response{
		Body: ioutil.NopCloser(bytes.NewReader([]byte(imageContent))),
	}
	client.On("Get", "http://foo.bar").Return(response, nil)

	l := new(mocks.LibvirtConnection)

	sp := libvirt.StoragePool{
		Name: poolName,
	}
	l.On("StoragePoolLookupByName", poolName).Return(sp, nil)

	sv := libvirt.StorageVol{
		Pool: poolName,
		Name: volName,
	}
	xml := fmt.Sprintf("t0 %v t1", volName)
	l.On("StorageVolCreateXML", sp, xml, mock.Anything).Return(sv, nil)

	l.On("StorageVolUpload",
		sv,
		readerMatcher([]byte(imageContent)),
		uint64(0),
		uint64(0),
		mock.Anything).Return(nil)

	v := New(l, poolName, directory)

	err := v.ImagePull(client, "http://foo.bar", volName)
	assert.NoError(t, err)

	client.AssertExpectations(t)
	l.AssertExpectations(t)
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
const imageContent = "some-data"
