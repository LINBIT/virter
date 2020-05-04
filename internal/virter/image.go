package virter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/LINBIT/virter/pkg/netcopy"
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

func (v *Virter) imageVolumeXML(name string) (string, error) {
	templateData := map[string]interface{}{
		"ImageName": name,
	}

	return v.renderTemplate(templateVolumeImage, templateData)
}

// ImageBuildTools includes the dependencies for building an image
type ImageBuildTools struct {
	ISOGenerator  ISOGenerator
	PortWaiter    PortWaiter
	DockerClient  DockerClient
	AfterNotifier AfterNotifier
}

// ImageBuildConfig contains the configuration for building an image
type ImageBuildConfig struct {
	DockerContainerConfig DockerContainerConfig
	SSHPrivateKeyPath     string
	SSHPrivateKey         []byte
	ShutdownTimeout       time.Duration
	ProvisionConfig       ProvisionConfig
}

// ImageBuild builds an image by running a VM and provisioning it
func (v *Virter) ImageBuild(ctx context.Context, tools ImageBuildTools, vmConfig VMConfig, buildConfig ImageBuildConfig) error {
	// VMRun is responsible to call CheckVMConfig here!
	err := v.VMRun(tools.ISOGenerator, tools.PortWaiter, vmConfig, true)
	if err != nil {
		return err
	}

	vmNames := []string{vmConfig.VMName}
	sshPrivateKey := buildConfig.SSHPrivateKey

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

	err = v.VMCommit(tools.AfterNotifier, vmConfig.VMName, true, buildConfig.ShutdownTimeout)
	if err != nil {
		return err
	}

	return nil
}

const templateVolumeImage = "volume-image.xml"
