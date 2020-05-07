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

	"github.com/stretchr/testify/assert"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery -name=HTTPClient

func TestImagePull(t *testing.T) {
	client := new(mocks.HTTPClient)
	mockGet(client, http.StatusOK)

	l := newFakeLibvirtConnection()

	v := virter.New(l, poolName, networkName)

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

	v := virter.New(l, poolName, networkName)

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

func TestImageBuild(t *testing.T) {
	shell := new(mocks.ShellClient)
	shell.On("Dial").Return(nil)
	shell.On("Close").Return(nil)

	docker := new(mocks.DockerClient)
	mockDockerRun(docker)

	an := new(mocks.AfterNotifier)
	mockAfter(an, make(chan time.Time))

	l := newFakeLibvirtConnection()

	l.vols[imageName] = &FakeLibvirtStorageVol{}
	l.lifecycleEvents = makeShutdownEvents()

	v := virter.New(l, poolName, networkName)

	tools := virter.ImageBuildTools{
		ShellClientBuilder: MockShellClientBuilder{shell},
		DockerClient:       docker,
		AfterNotifier:      an,
	}

	vmConfig := virter.VMConfig{
		ImageName:     imageName,
		Name:          vmName,
		ID:            vmID,
		MemoryKiB:     1024,
		VCPUs:         1,
		SSHPublicKeys: []string{sshPublicKey},
		SSHPrivateKey: []byte(sshPrivateKey),
		WaitSSH:       true,
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

	dockercfg := virter.DockerContainerConfig{}
	buildConfig := virter.ImageBuildConfig{
		DockerContainerConfig: dockercfg,
		SSHPrivateKey:         []byte(sshPrivateKey),
		ShutdownTimeout:       shutdownTimeout,
		ProvisionConfig:       provisionConfig,
	}

	err := v.ImageBuild(context.Background(), tools, vmConfig, buildConfig)
	assert.NoError(t, err)

	assert.Len(t, l.vols, 2)
	assert.Empty(t, l.network.description.IPs[0].DHCP.Hosts)
	assert.Empty(t, l.domains)

	shell.AssertExpectations(t)
	docker.AssertExpectations(t)
	an.AssertExpectations(t)
}

const imageURL = "http://foo.bar"
const imageContent = "some-data"
