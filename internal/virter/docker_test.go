package virter_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/LINBIT/containerapi"
	"github.com/stretchr/testify/assert"
)

type MockContainerProvider struct {
	createCalled bool
	startCalled  bool
	logsCalled   bool
	waitCalled   bool
}

const mockContainerId = "some-container-id"

func (c *MockContainerProvider) Create(ctx context.Context, cfg *containerapi.ContainerConfig) (string, error) {
	c.createCalled = true
	return mockContainerId, nil
}

func (c *MockContainerProvider) Start(ctx context.Context, containerID string) error {
	c.startCalled = true
	return nil
}

func (c *MockContainerProvider) Stop(ctx context.Context, containerID string, timeout *time.Duration) error {
	return fmt.Errorf("called Stop on mock container")
}

func (c *MockContainerProvider) Wait(ctx context.Context, containerID string) (<-chan int64, <-chan error) {
	c.waitCalled = true
	statusChan := make(chan int64, 1)
	statusChan <- 0
	errChan := make(chan error)
	return statusChan, errChan
}

func (c *MockContainerProvider) Logs(ctx context.Context, containerID string) (io.ReadCloser, io.ReadCloser, error) {
	c.logsCalled = true
	var out bytes.Buffer
	return ioutil.NopCloser(&out), ioutil.NopCloser(&out), nil
}

func (c *MockContainerProvider) Remove(ctx context.Context, containerID string) error {
	return nil
}

func (c *MockContainerProvider) CopyFrom(ctx context.Context, containerId string, source string, dest string) error {
	return nil
}

func (c *MockContainerProvider) Close() error {
	return nil
}

func (c *MockContainerProvider) AssertExpectations(t *testing.T) {
	assert.True(t, c.createCalled)
	assert.True(t, c.startCalled)
	assert.True(t, c.waitCalled)
	assert.True(t, c.logsCalled)
}

func mockContainerProvider() *MockContainerProvider {
	return &MockContainerProvider{}
}
