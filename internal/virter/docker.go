package virter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerClient contains the required docker methods.
type DockerClient interface {
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error)
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.ContainerWaitOKBody, <-chan error)
	ContainerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error)
}

// DockerContainerConfig contains the configuration for a to be started container
type DockerContainerConfig struct {
	ContainerName string   // the name of the container
	ImageName     string   // the name of the container image
	Env           []string // the environment (variables) passed to the container
}

func dockerRun(ctx context.Context, docker DockerClient, dockerContainerConfig DockerContainerConfig, vmIPs []string, sshPrivateKey []byte) error {
	// This is roughly equivalent to
	// docker run --rm --network=host -e TARGETS=$vmIPs -e SSH_PRIVATE_KEY="$sshPrivateKey" $dockerImageName

	targetEnv := fmt.Sprintf("TARGETS=%s", strings.Join(vmIPs, ","))
	sshPrivateKeyEnv := fmt.Sprintf("SSH_PRIVATE_KEY=%s", sshPrivateKey)

	resp, err := docker.ContainerCreate(
		ctx,
		&container.Config{
			Image: dockerContainerConfig.ImageName,
			Env:   append(dockerContainerConfig.Env, targetEnv, sshPrivateKeyEnv),
		},
		&container.HostConfig{
			NetworkMode: "host",
			AutoRemove:  true,
		},
		nil,
		dockerContainerConfig.ContainerName)
	if err != nil {
		return fmt.Errorf("could not create container: %w", err)
	}

	statusCh, errCh := docker.ContainerWait(ctx, resp.ID, container.WaitConditionRemoved)

	err = docker.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("could not start container: %w", err)
	}

	err = dockerStreamLogs(ctx, docker, resp.ID)
	if err != nil {
		return err
	}

	err = dockerContainerWait(ctx, statusCh, errCh)
	if err != nil {
		return err
	}

	return nil
}

func dockerStreamLogs(ctx context.Context, docker DockerClient, id string) error {
	out, err := docker.ContainerLogs(
		ctx, id,
		types.ContainerLogsOptions{
			ShowStdout: true, ShowStderr: true, Follow: true,
		})
	if err != nil {
		return fmt.Errorf("could not get container logs: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	go logLines(&wg, "Docker stdout: ", stdoutReader)
	go logLines(&wg, "Docker stderr: ", stderrReader)

	_, err = stdcopy.StdCopy(stdoutWriter, stderrWriter, out)
	if err != nil {
		return fmt.Errorf("error copying container output: %w", err)
	}

	stdoutWriter.Close()
	stderrWriter.Close()

	wg.Wait()
	return nil
}

func logLines(wg *sync.WaitGroup, prefix string, r io.Reader) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		log.Printf("%s%s", prefix, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("%sError reading: %v", prefix, err)
	}
}

func dockerContainerWait(ctx context.Context, statusCh <-chan container.ContainerWaitOKBody, errCh <-chan error) error {
	select {
	case err := <-errCh:
		return fmt.Errorf("error waiting for container: %w", err)
	case status := <-statusCh:
		if status.Error != nil {
			return fmt.Errorf("error from container '%s' (exit code %d)", status.Error.Message, status.StatusCode)
		}
		if status.StatusCode != 0 {
			return fmt.Errorf("container returned non-zero exit code %d", status.StatusCode)
		}
	}
	return nil
}
