package virter_test

import (
	"bytes"
	"io/ioutil"

	"github.com/stretchr/testify/mock"

	"github.com/docker/docker/api/types/container"

	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery -name=DockerClient

func mockDockerRun(docker *mocks.DockerClient) {
	id := "some-container-id"

	createBody := container.ContainerCreateCreatedBody{ID: id}
	docker.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(createBody, nil)

	docker.On("ContainerStart", mock.Anything, id, mock.Anything).Return(nil)

	var out bytes.Buffer
	docker.On("ContainerLogs", mock.Anything, id, mock.Anything).Return(ioutil.NopCloser(&out), nil)

	statusCh := make(chan container.ContainerWaitOKBody, 1)
	statusCh <- container.ContainerWaitOKBody{}
	errCh := make(chan error)
	mockContainerWait(docker, id, statusCh, errCh)
}

// mockContainerWait wraps the ContainerWait mocking so that the channels have
// the correct "<-chan" type instead of "chan"
func mockContainerWait(docker *mocks.DockerClient, id string, statusCh <-chan container.ContainerWaitOKBody, errCh <-chan error) {
	docker.On("ContainerWait", mock.Anything, id, container.WaitConditionNotRunning).Return(statusCh, errCh)
}
