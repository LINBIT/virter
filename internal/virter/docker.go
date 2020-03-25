package virter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"sync"

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

func dockerRun(ctx context.Context, docker DockerClient, dockerImageName string, vmName string, vmIP string, sshPrivateKey []byte) error {
	// This is roughly equivalent to
	// docker run --rm --network=host -e TARGET=$vmIP -e SSH_PRIVATE_KEY="$sshPrivateKey" $dockerImageName

	targetEnv := fmt.Sprintf("TARGET=%s", vmIP)
	sshPrivateKeyEnv := fmt.Sprintf("SSH_PRIVATE_KEY=%s", sshPrivateKey)
	containerName := fmt.Sprintf("virter-%s", vmName)

	resp, err := docker.ContainerCreate(
		ctx,
		&container.Config{
			Image: dockerImageName,
			Env:   []string{targetEnv, sshPrivateKeyEnv},
		},
		&container.HostConfig{
			NetworkMode: "host",
			AutoRemove:  true,
		},
		nil,
		containerName)
	if err != nil {
		return fmt.Errorf("could not create container: %w", err)
	}

	err = docker.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("could not start container: %w", err)
	}

	err = dockerStreamLogs(ctx, docker, resp.ID)
	if err != nil {
		return err
	}

	err = dockerContainerWait(ctx, docker, resp.ID)
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

func dockerContainerWait(ctx context.Context, docker DockerClient, id string) error {
	statusCh, errCh := docker.ContainerWait(ctx, id, container.WaitConditionNotRunning)
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
