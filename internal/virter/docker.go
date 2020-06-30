package virter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerClient contains the required docker methods.
type DockerClient interface {
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error)
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
	ContainerStop(ctx context.Context, containerID string, timeout *time.Duration) error
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.ContainerWaitOKBody, <-chan error)
	ContainerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error)
}

// DockerContainerConfig contains the configuration for a to be started container
type DockerContainerConfig struct {
	ContainerName string   // the name of the container
	ImageName     string   // the name of the container image
	Env           []string // the environment (variables) passed to the container
}

const (
	colorDefault = colorReset
	colorRed     = "\u001b[31m"
	colorReset   = "\u001b[0m"
)

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

	// dockerStreamLogs is ctx safe (i.e., errs out in copy if ctx cancled)
	err = dockerStreamLogs(ctx, docker, resp.ID)
	if err != nil { // something weird happened here, most likely context canceled
		td := 200 * time.Millisecond // this horse is already dead...
		if stopErr := docker.ContainerStop(context.Background(), resp.ID, &td); stopErr != nil {
			return fmt.Errorf("could not stop container: %v, after log streaming failed: %w", stopErr, err)
		}
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

	go logLines(&wg, "Docker", false, stdoutReader)
	go logLines(&wg, "Docker", true, stderrReader)

	_, err = stdcopy.StdCopy(stdoutWriter, stderrWriter, out)
	if err != nil {
		return fmt.Errorf("error copying container output: %w", err)
	}

	stdoutWriter.Close()
	stderrWriter.Close()

	wg.Wait()
	return nil
}

// logStdoutStderr logs a message from a VM which came from either stdout or stderr
func logStdoutStderr(vmName, message string, stderr bool) {
	var prefix string
	var color string
	if stderr {
		prefix = "err"
		color = colorRed
	} else {
		prefix = "out"
		color = colorDefault
	}

	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		message = color + message + colorReset
	}

	log.Printf("%s %s: %s", vmName, prefix, message)
}

func logLines(wg *sync.WaitGroup, vm string, stderr bool, r io.Reader) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		message := strings.TrimRight(scanner.Text(), " \t\r\n")
		logStdoutStderr(vm, message, stderr)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("%s: Error reading: %v", vm, err)
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
