package virter_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	libvirtxml "github.com/libvirt/libvirt-go-xml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery --name=HTTPClient

func TestImagePull(t *testing.T) {
	client := new(mocks.HTTPClient)
	mockDo(client, http.StatusOK)

	l := newFakeLibvirtConnection()

	v := virter.New(l, poolName, networkName, newMockKeystore())

	ctx := context.Background()
	err := v.ImagePull(ctx, client, nopReaderProxy{}, imageURL, imageName)
	assert.NoError(t, err)

	client.AssertExpectations(t)

	assert.Len(t, l.vols, 1)
	assert.Equal(t, []byte(imageContent), l.vols[imageName].content)
}

func TestImagePullBadStatus(t *testing.T) {
	client := new(mocks.HTTPClient)
	mockDo(client, http.StatusNotFound)

	l := newFakeLibvirtConnection()

	v := virter.New(l, poolName, networkName, newMockKeystore())

	ctx := context.Background()
	err := v.ImagePull(ctx, client, nopReaderProxy{}, imageURL, imageName)
	assert.Error(t, err)

	client.AssertExpectations(t)

	assert.Empty(t, l.vols)
}

func mockDo(client *mocks.HTTPClient, statusCode int) {
	response := &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("Status: %v", statusCode),
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(imageContent))),
	}
	req, _ := http.NewRequest("GET", imageURL, nil)
	client.On("Do", req).Return(response, nil)
}

type nopReaderProxy struct {
}

func (b nopReaderProxy) SetTotal(total int64) {
}

func (b nopReaderProxy) ProxyReader(r io.ReadCloser) io.ReadCloser {
	return r
}

func TestImageBuild(t *testing.T) {
	shell := new(mocks.ShellClient)
	shell.On("DialContext", mock.Anything).Return(nil)
	shell.On("Close").Return(nil)

	docker := mockContainerProvider()

	an := new(mocks.AfterNotifier)
	mockAfter(an, make(chan time.Time))

	l := newFakeLibvirtConnection()

	l.vols[imageName] = &FakeLibvirtStorageVol{}

	v := virter.New(l, poolName, networkName, newMockKeystore())

	tools := virter.ImageBuildTools{
		ShellClientBuilder: MockShellClientBuilder{shell},
		ContainerProvider:  docker,
		AfterNotifier:      an,
	}

	vmConfig := virter.VMConfig{
		ImageName:          imageName,
		Name:               vmName,
		ID:                 vmID,
		StaticDHCP:         false,
		MemoryKiB:          1024,
		VCPUs:              1,
		ExtraSSHPublicKeys: []string{sshPublicKey},
	}

	sshPingConfig := virter.SSHPingConfig{
		SSHPingCount:  1,
		SSHPingPeriod: time.Second, // ignored
	}

	provisionConfig := virter.ProvisionConfig{
		Steps: []virter.ProvisionStep{
			virter.ProvisionStep{
				Docker: &virter.ProvisionDockerStep{
					Image: dockerImageName,
				},
			},
		},
	}

	buildConfig := virter.ImageBuildConfig{
		ContainerName:   "virter-test",
		ShutdownTimeout: shutdownTimeout,
		ProvisionConfig: provisionConfig,
	}

	err := v.ImageBuild(context.Background(), tools, vmConfig, sshPingConfig, buildConfig)
	assert.NoError(t, err)

	assert.Len(t, l.vols, 2)
	assert.Empty(t, l.networks[networkName].description.IPs[0].DHCP.Hosts)
	assert.Empty(t, l.domains)

	shell.AssertExpectations(t)
	docker.AssertExpectations(t)
	an.AssertExpectations(t)
}

func TestImageSave(t *testing.T) {
	l := newFakeLibvirtConnection()

	l.vols[imageName] = &FakeLibvirtStorageVol{
		description: &libvirtxml.StorageVolume{
			Name:   imageName,
			Target: &libvirtxml.StorageVolumeTarget{},
			Physical: &libvirtxml.StorageVolumeSize{
				Value: uint64(len(imageContent)),
			},
		},
		content: []byte(imageContent),
	}

	v := virter.New(l, poolName, networkName, newMockKeystore())

	var dest bytes.Buffer
	err := v.ImageSave(imageName, &dest)
	assert.NoError(t, err)

	assert.Len(t, l.vols, 1)
	assert.Equal(t, []byte(imageContent), dest.Bytes())
}

const imageURL = "http://foo.bar"
const imageContent = "some-data"
