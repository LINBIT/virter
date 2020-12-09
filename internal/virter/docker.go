package virter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LINBIT/virter/pkg/overtime"
	"github.com/LINBIT/virter/pkg/sshkeys"

	"github.com/LINBIT/containerapi"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	colorDefault = colorReset
	colorRed     = "\u001b[31m"
	colorReset   = "\u001b[0m"
)

func containerRun(ctx context.Context, containerProvider containerapi.ContainerProvider, containerCfg *containerapi.ContainerConfig, vmNames []string, keyStore sshkeys.KeyStore, knownHosts sshkeys.KnownHosts, copyStep *ProvisionDockerCopyStep) error {
	// This is roughly equivalent to
	// docker run --rm --network=host -e TARGETS=$vmIPs -e SSH_PRIVATE_KEY="$sshPrivateKey" $dockerImageName
	cleanupContext, cleanupCancel := overtime.WithOvertimeContext(ctx, 10*time.Second)
	defer cleanupCancel()

	knownHostsFile, err := ioutil.TempFile("", "virter-container-known-hosts-*")
	if err != nil {
		return fmt.Errorf("failed to create known hosts file: %w", err)
	}

	defer os.Remove(knownHostsFile.Name())

	err = knownHosts.AsKnownHostsFile(knownHostsFile)
	if err != nil {
		return fmt.Errorf("failed to write known hosts file: %w", err)
	}

	err = knownHostsFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close known hosts file: %w", err)
	}

	containerCfg.AddMount(containerapi.Mount{HostPath: keyStore.KeyPath(), ContainerPath: "/root/.ssh/id_rsa", ReadOnly: true})
	containerCfg.AddMount(containerapi.Mount{HostPath: knownHostsFile.Name(), ContainerPath: "/root/.ssh/known_hosts"})

	containerCfg.SetEnv("TARGETS", strings.Join(vmNames, ","))
	containerCfg.SetEnv("SSH_PRIVATE_KEY", string(keyStore.KeyBytes()))

	containerID, err := containerProvider.Create(
		ctx,
		containerCfg,
	)
	if err != nil {
		return fmt.Errorf("could not create container: %w", err)
	}

	defer func() {
		err := containerProvider.Remove(cleanupContext, containerID)
		if err != nil {
			log.WithFields(log.Fields{"err": err, "container": containerID}).Warn("failed to remove container")
		}
	}()

	statusCh, errCh := containerProvider.Wait(ctx, containerID)

	// Note: With incredible (bad) luck, you can start a container but cancel the context just in time to not get a
	// successful response on Start(). Since Stop() is idempotent, we can just defer it before the Start() call.
	defer func() {
		killTimeout := 200 * time.Millisecond
		err := containerProvider.Stop(cleanupContext, containerID, &killTimeout)
		if err != nil {
			log.WithFields(log.Fields{"err": err, "container": containerID}).Warn("failed to stop container")
		}
	}()
	err = containerProvider.Start(ctx, containerID)
	if err != nil {
		return fmt.Errorf("could not start container: %w", err)
	}

	err = streamLogs(ctx, containerProvider, containerID)
	if err != nil {
		return err
	}

	waitErr := containerWait(statusCh, errCh)

	// Copy out files from container even if the wait ended in an error.
	// The files may still be important. For instance, when the wait timed
	// out or the container terminated with a non-zero exit code. This
	// generally indicates a failure of the process running in the
	// container. In these cases, the container itself is still valid.
	if copyStep != nil {
		// Use a fresh Context here because ctx may have been canceled
		copyCtx, copyCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer copyCancel()
		err = containerCopy(copyCtx, containerProvider, containerID, copyStep)
		if err != nil {
			return err
		}
	}

	return waitErr
}

func streamLogs(ctx context.Context, containerProvider containerapi.ContainerProvider, id string) error {
	stdout, stderr, err := containerProvider.Logs(ctx, id)
	if err != nil {
		return fmt.Errorf("could not get container logs: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go logLines(&wg, "Docker", false, stdout)
	go logLines(&wg, "Docker", true, stderr)

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

func containerWait(statusCh <-chan int64, errCh <-chan error) error {
	select {
	case err := <-errCh:
		return fmt.Errorf("error waiting for container: %w", err)
	case status := <-statusCh:
		if status != 0 {
			return fmt.Errorf("container returned non-zero exit code %d", status)
		}
		return nil
	}
}

func containerCopy(ctx context.Context, provider containerapi.ContainerProvider, containerID string, step *ProvisionDockerCopyStep) error {
	destDir, err := filepath.Abs(step.Dest)
	if err != nil {
		return fmt.Errorf("failed to determine absolute path of destination %q: %w", step.Dest, err)
	}
	return provider.CopyFrom(ctx, containerID, step.Source, destDir)
}
