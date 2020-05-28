package virter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/LINBIT/virter/pkg/netcopy"
	log "github.com/sirupsen/logrus"
)

// HTTPClient contains required HTTP methods.
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// ReaderProxy wraps reading from a Reader with a known total size.
type ReaderProxy interface {
	SetTotal(total int64)
	ProxyReader(r io.ReadCloser) io.ReadCloser
}

// ImagePull pulls an image from a URL into libvirt.
func (v *Virter) ImagePull(client HTTPClient, readerProxy ReaderProxy, url string, name string) error {
	xml, err := v.imageVolumeXML(name)
	if err != nil {
		return err
	}

	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get from %v: %w", url, err)
	}
	readerProxy.SetTotal(response.ContentLength)
	proxyResponse := readerProxy.ProxyReader(response.Body)
	defer proxyResponse.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("error %v from %v", response.Status, url)
	}

	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	sv, err := v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create storage volume: %w", err)
	}

	err = v.libvirt.StorageVolUpload(sv, proxyResponse, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to transfer data from URL to libvirt: %w", err)
	}

	return nil
}

// ImageBuildTools includes the dependencies for building an image
type ImageBuildTools struct {
	ShellClientBuilder ShellClientBuilder
	DockerClient       DockerClient
	AfterNotifier      AfterNotifier
}

// ImageBuildConfig contains the configuration for building an image
type ImageBuildConfig struct {
	DockerContainerConfig DockerContainerConfig
	SSHPrivateKeyPath     string
	SSHPrivateKey         []byte
	ShutdownTimeout       time.Duration
	ProvisionConfig       ProvisionConfig
	ResetMachineID        bool
}

func (v *Virter) imageBuildProvisionCommit(ctx context.Context, tools ImageBuildTools, vmConfig VMConfig, buildConfig ImageBuildConfig) error {
	vmNames := []string{vmConfig.Name}
	sshPrivateKey := buildConfig.SSHPrivateKey
	var err error

	if buildConfig.ResetMachineID {
		// starting the VM creates a machine-id
		// we want these IDs to be unique, so reset to empty
		resetMachineID := ProvisionStep{
			Shell: &ProvisionShellStep{
				Script: "truncate -c -s 0 /etc/machine-id",
			},
		}
		buildConfig.ProvisionConfig.Steps = append(buildConfig.ProvisionConfig.Steps, resetMachineID)
	}

	for _, s := range buildConfig.ProvisionConfig.Steps {
		if s.Docker != nil {
			dockerContainerConfig := buildConfig.DockerContainerConfig
			dockerContainerConfig.ImageName = s.Docker.Image
			dockerContainerConfig.Env = EnvmapToSlice(s.Docker.Env)
			err = v.VMExecDocker(ctx, tools.DockerClient, vmNames, dockerContainerConfig, sshPrivateKey)
		} else if s.Shell != nil {
			err = v.VMExecShell(ctx, vmNames, sshPrivateKey, s.Shell)
		} else if s.Rsync != nil {
			copier := netcopy.NewRsyncNetworkCopier(buildConfig.SSHPrivateKeyPath)
			err = v.VMExecRsync(ctx, copier, vmNames, s.Rsync)
		}

		if err != nil {
			return err
		}
	}

	err = v.VMCommit(tools.AfterNotifier, vmConfig.Name, true, buildConfig.ShutdownTimeout)
	if err != nil {
		return err
	}

	return nil
}

// ImageBuild builds an image by running a VM and provisioning it
func (v *Virter) ImageBuild(ctx context.Context, tools ImageBuildTools, vmConfig VMConfig, buildConfig ImageBuildConfig) error {
	// VMRun is responsible to call CheckVMConfig here!
	// TODO(): currently we can not know why VM run failed, so we don't clean up in this stage,
	//         it could have been an existing VM, we don't want to delete it.
	err := v.VMRun(tools.ShellClientBuilder, vmConfig)
	if err != nil {
		return err
	}

	// from here on it is safe to rm the VM if something fails
	err = v.imageBuildProvisionCommit(ctx, tools, vmConfig, buildConfig)
	if err != nil {
		log.Warn("could not build image, deleting VM")
		if rmErr := v.VMRm(vmConfig.Name); rmErr != nil {
			return fmt.Errorf("could not delete VM: %v, after build failed: %w", rmErr, err)
		}
		return err
	}

	return nil
}
