package virter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/LINBIT/containerapi"

	"github.com/LINBIT/virter/pkg/netcopy"
	libvirt "github.com/digitalocean/go-libvirt"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

// HTTPClient contains required HTTP methods.
type HTTPClient interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

// ReaderProxy wraps reading from a Reader with a known total size.
type ReaderProxy interface {
	SetTotal(total int64)
	ProxyReader(r io.ReadCloser) io.ReadCloser
}

// ImagePull pulls an image from a URL into libvirt.
func (v *Virter) ImagePull(ctx context.Context, client HTTPClient, readerProxy ReaderProxy, url, name string) error {
	xml, err := v.imageVolumeXML(name)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(req)
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
		err = fmt.Errorf("failed to transfer data from URL to libvirt: %w", err)
		if rmErr := v.rmVolume(sp, name, name); rmErr != nil {
			err = fmt.Errorf("could not remove image: %v, after transfer failed: %w", rmErr, err)
		}
		return err
	}

	return nil
}

// ImageRm removes an image from libvirt.
func (v *Virter) ImageRm(ctx context.Context, name string) error {
	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	return v.rmVolume(sp, name, name)
}

// ImageBuildTools includes the dependencies for building an image
type ImageBuildTools struct {
	ShellClientBuilder ShellClientBuilder
	ContainerProvider  containerapi.ContainerProvider
	AfterNotifier      AfterNotifier
}

// ImageBuildConfig contains the configuration for building an image
type ImageBuildConfig struct {
	ContainerName     string
	SSHPrivateKeyPath string
	SSHPrivateKey     []byte
	ShutdownTimeout   time.Duration
	ProvisionConfig   ProvisionConfig
	ResetMachineID    bool
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
			containerCfg := containerapi.NewContainerConfig(buildConfig.ContainerName, s.Docker.Image, s.Docker.Env)
			err = v.VMExecDocker(ctx, tools.ContainerProvider, vmNames, containerCfg, sshPrivateKey)
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

func (v *Virter) volDeleteMust(vol libvirt.StorageVol) {
	err := v.libvirt.StorageVolDelete(vol, 0)
	if err != nil {
		log.Errorf("Failed to delete storage volume: %v", err)
	}
}

func (v *Virter) ImageSave(name string, to io.Writer) error {
	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	vol, err := v.libvirt.StorageVolLookupByName(sp, name)
	if err != nil {
		return fmt.Errorf("could not get storage volume: %w", err)
	}

	oldXML, err := v.libvirt.StorageVolGetXMLDesc(vol, 0)
	if err != nil {
		return fmt.Errorf("could not get storage volume XML: %w", err)
	}

	volcfg := &libvirtxml.StorageVolume{}
	err = volcfg.Unmarshal(oldXML)
	if err != nil {
		return fmt.Errorf("could not unmarshal storage volume XML: %w", err)
	}

	volcfg.Name = volcfg.Name + "-clone-" + uuid.NewV4().String()

	newXML, err := volcfg.Marshal()
	if err != nil {
		return fmt.Errorf("could not marshal storage volume XML: %w", err)
	}

	newVol, err := v.libvirt.StorageVolCreateXMLFrom(sp, newXML, vol, 0)
	if err != nil {
		return fmt.Errorf("could not clone volume: %w", err)
	}
	defer v.volDeleteMust(newVol)

	err = v.libvirt.StorageVolDownload(newVol, to, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("could not download volume: %w", err)
	}

	return nil
}
