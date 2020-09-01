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

	"github.com/LINBIT/containerapi"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	colorDefault = colorReset
	colorRed     = "\u001b[31m"
	colorReset   = "\u001b[0m"
)

func containerRun(ctx context.Context, containerProvider containerapi.ContainerProvider, containerCfg *containerapi.ContainerConfig, vmIPs []string, sshPrivateKey []byte) error {
	// This is roughly equivalent to
	// docker run --rm --network=host -e TARGETS=$vmIPs -e SSH_PRIVATE_KEY="$sshPrivateKey" $dockerImageName

	containerCfg.SetEnv("TARGETS", strings.Join(vmIPs, ","))
	containerCfg.SetEnv("SSH_PRIVATE_KEY", string(sshPrivateKey))

	resp, err := containerProvider.Create(
		ctx,
		containerCfg,
	)
	if err != nil {
		return fmt.Errorf("could not create container: %w", err)
	}

	statusCh, errCh := containerProvider.Wait(ctx, resp)

	err = containerProvider.Start(ctx, resp)
	if err != nil {
		return fmt.Errorf("could not start container: %w", err)
	}

	// streamLogs is ctx safe (i.e., errs out in copy if ctx cancled)
	err = streamLogs(ctx, containerProvider, resp)
	if err != nil { // something weird happened here, most likely context canceled
		td := 200 * time.Millisecond // this horse is already dead...
		if stopErr := containerProvider.Stop(context.Background(), resp, &td); stopErr != nil {
			return fmt.Errorf("could not stop container: %v, after log streaming failed: %w", stopErr, err)
		}
		return err
	}

	err = containerWait(ctx, statusCh, errCh)
	if err != nil {
		return err
	}

	return nil
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

func containerWait(ctx context.Context, statusCh <-chan int64, errCh <-chan error) error {
	select {
	case <- ctx.Done():
		return fmt.Errorf("timeout waiting for container: %v", ctx.Err())
	case err := <-errCh:
		return fmt.Errorf("error waiting for container: %w", err)
	case status := <-statusCh:
		if status != 0 {
			return fmt.Errorf("container returned non-zero exit code %d", status)
		}
	}
	return nil
}
