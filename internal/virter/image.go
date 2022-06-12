// Work with
//

package virter

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/LINBIT/containerapi"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v7"

	"github.com/LINBIT/virter/pkg/netcopy"
)

const (
	RootFSType      = "application/vnd.com.linbit.virter.image.v1.rootfs"
	ImageMediaType  = "application/vnd.com.linbit.virter.image.v1"
	TagVolumePrefix = "virter:tag:"
)

// HTTPClient contains required HTTP methods.
type HTTPClient interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

// LocalImage represents a list of local layers addressable by a human readable name.
//
// An image is stored in libvirt as an empty volume prefixed by TagVolumePrefix. This volume has the actual VolumeLayer
// as backing store. This type implements the regv1.Image interface, i.e. it can be pushed to a container registry.
type LocalImage struct {
	tagLayer *RawLayer
	topLayer *VolumeLayer
	lock     sync.Mutex
	layers   []regv1.Layer
	opts     []LayerOperationOption
}

// Ensure LocalImage satisfies the interface.
var _ regv1.Image = &LocalImage{}

// Layers returns the ordered list of layers that make up this image.
//
// The order of layers is oldest/base layer first, most recent/top layer last.
// Since VolumeLayer are not directly suitable for pushing to a registry, this method pre-computes the compatible
// layers and caches the result for later use.
func (l *LocalImage) Layers() ([]regv1.Layer, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if len(l.layers) != 0 {
		return l.layers, nil
	}

	var layers []regv1.Layer
	cur := l.topLayer
	for {
		if cur == nil {
			l.layers = layers
			return l.layers, nil
		}

		compat, err := cur.ToRegistryLayer(l.opts...)
		if err != nil {
			return nil, err
		}

		layers = append([]regv1.Layer{compat}, layers...)
		dep, err := cur.Dependency()
		if err != nil {
			return nil, err
		}

		cur = dep
	}
}

// MediaType of this image's manifest.
func (l *LocalImage) MediaType() (types.MediaType, error) {
	return types.DockerManifestSchema2, nil
}

// Size returns the size of the manifest.
func (l *LocalImage) Size() (int64, error) {
	return partial.Size(l)
}

// ConfigName returns the hash of the image's config file, also known as the Image ID.
func (l *LocalImage) ConfigName() (regv1.Hash, error) {
	return partial.ConfigName(l)
}

// ConfigFile returns this image's config file.
//
// For containers this specifies metadata like which command to run and permissions. For us this simply carries
// the list layers by their uncompressed id.
func (l *LocalImage) ConfigFile() (*regv1.ConfigFile, error) {
	layers, err := l.Layers()
	if err != nil {
		return nil, err
	}

	ids := make([]regv1.Hash, len(layers))
	for i := range layers {
		digest, err := layers[i].DiffID()
		if err != nil {
			return nil, err
		}
		ids[i] = digest
	}

	return &regv1.ConfigFile{
		RootFS: regv1.RootFS{
			Type:    RootFSType,
			DiffIDs: ids,
		},
	}, nil
}

// RawConfigFile returns the serialized bytes of ConfigFile().
func (l *LocalImage) RawConfigFile() ([]byte, error) {
	return partial.RawConfigFile(l)
}

// Digest returns the sha256 of this image's manifest.
func (l *LocalImage) Digest() (regv1.Hash, error) {
	return partial.Digest(l)
}

// Manifest returns this image's Manifest object.
//
// The manifest ties a tag to layers. When fetching an image from a registry, first the matching manifest is fetched.
// The manifest references the layers to fetch and the (container) configuration. For our purpose, only the layers
// are of interest.
func (l *LocalImage) Manifest() (*regv1.Manifest, error) {
	raw, err := l.RawConfigFile()
	if err != nil {
		return nil, err
	}

	cfgHash, cfgSize, err := regv1.SHA256(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}

	layers, err := l.Layers()
	if err != nil {
		return nil, err
	}

	layerDescriptors := make([]regv1.Descriptor, len(layers))
	for i := range layers {
		desc, err := partial.Descriptor(layers[i])
		if err != nil {
			return nil, err
		}

		layerDescriptors[i] = *desc
	}

	return &regv1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.DockerManifestSchema2,
		Config: regv1.Descriptor{
			MediaType: ImageMediaType,
			Size:      cfgSize,
			Digest:    cfgHash,
		},
		Layers: layerDescriptors,
	}, nil
}

// RawManifest returns the serialized bytes of Manifest()
func (l *LocalImage) RawManifest() ([]byte, error) {
	return partial.RawManifest(l)
}

// LayerByDigest returns a Layer for interacting with a particular layer of
// the image, looking it up by "digest" (the compressed hash).
func (l *LocalImage) LayerByDigest(hash regv1.Hash) (regv1.Layer, error) {
	layers, err := l.Layers()
	if err != nil {
		return nil, err
	}

	for _, l := range layers {
		id, err := l.Digest()
		if err != nil {
			return nil, err
		}

		if id == hash {
			return l, nil
		}
	}

	return nil, nil
}

// LayerByDiffID is an analog to LayerByDigest, looking up by "diff id"
// (the uncompressed hash).
func (l *LocalImage) LayerByDiffID(hash regv1.Hash) (regv1.Layer, error) {
	layers, err := l.Layers()
	if err != nil {
		return nil, err
	}

	for _, l := range layers {
		id, err := l.DiffID()
		if err != nil {
			return nil, err
		}

		if id == hash {
			return l, nil
		}
	}

	return nil, nil
}

// TopLayer returns the top most VolumeLayer.
//
// The returned volume layer can be used as backing volume when starting a new VM based on this image.
func (l *LocalImage) TopLayer() *VolumeLayer {
	return l.topLayer
}

// Name returns the local name of the image.
func (l *LocalImage) Name() string {
	return strings.TrimPrefix(l.tagLayer.volume.Name, TagVolumePrefix)
}

// ImageSpawn creates a new rw volume backed by this image.
//
// The returned RawLayer can be attached to a VM.
func (v *Virter) ImageSpawn(name string, image *LocalImage, capacityKib uint64) (*RawLayer, error) {
	raw, err := v.NewDynamicLayer(name, WithBackingLayer(image.topLayer), WithCapacity(capacityKib))
	if err != nil {
		return nil, err
	}

	return raw, nil
}

// FindImage searches the default storage pool for a LocalImage of the given name.
//
// If no matching image could be found, returns (nil, nil).
func (v *Virter) FindImage(image string, opts ...LayerOperationOption) (*LocalImage, error) {
	rawName := TagVolumePrefix + image
	raw, err := v.FindRawLayer(rawName)
	if err != nil {
		return nil, err
	}

	if raw == nil {
		return nil, nil
	}

	top, err := raw.Dependency()
	if err != nil {
		return nil, err
	}

	return &LocalImage{
		tagLayer: raw,
		topLayer: top,
		opts:     opts,
	}, nil
}

// MakeImage creates a new LocalImage of the given name, pointing to the given VolumeLayer.
//
// If an image of the given name already exists it will be deleted first.
func (v *Virter) MakeImage(image string, topLayer *VolumeLayer, opts ...LayerOperationOption) (*LocalImage, error) {
	rawName := TagVolumePrefix + image
	existing, err := v.FindRawLayer(rawName)
	if err != nil {
		return nil, fmt.Errorf("failed to check for existing image '%s': %w", image, err)
	}

	if existing != nil {
		dep, err := existing.Dependency()
		if err != nil {
			return nil, err
		}

		depDigest, err := dep.DiffID()
		if err != nil {
			return nil, err
		}

		newDigest, err := topLayer.DiffID()
		if err != nil {
			return nil, err
		}

		if depDigest == newDigest {
			return &LocalImage{
				tagLayer: existing,
				topLayer: topLayer,
			}, nil
		}

		err = existing.DeleteAllIfUnused()
		if err != nil {
			return nil, err
		}
	}

	raw, err := v.emptyVolume(rawName, WithBackingLayer(topLayer))
	if err != nil {
		return nil, err
	}

	tagLayer := &RawLayer{
		volume: raw,
		pool:   v.provisionStoragePool,
	}

	return &LocalImage{
		tagLayer: tagLayer,
		topLayer: topLayer,
		opts:     opts,
	}, nil
}

// ImageImport imports the given registry image into the local storage pool
func (v *Virter) ImageImport(name string, image regv1.Image, opts ...LayerOperationOption) (*LocalImage, error) {
	o := makeLayerOperationOpts(opts...)

	manifest, err := image.Manifest()
	if err != nil {
		return nil, fmt.Errorf("error checking media type: %w", err)
	}

	if manifest.Config.MediaType != ImageMediaType {
		return nil, &nonVirterVolumeError{name: name}
	}

	layers, err := image.Layers()
	if err != nil {
		return nil, fmt.Errorf("error getting image layers")
	}

	var topLayer *VolumeLayer
	for _, layer := range layers {
		diffId, err := layer.DiffID()
		if err != nil {
			return nil, fmt.Errorf("error checking diff id: %w", err)
		}

		mediaType, err := layer.MediaType()
		if err != nil {
			return nil, fmt.Errorf("error checking media type: %w", err)
		}

		if mediaType != LayerMediaType {
			return nil, fmt.Errorf("%s is not a virter layer", diffId.String())
		}

		var bar *mpb.Bar
		size := int64(0)
		if o.Progress != nil {
			size, err = layer.Size()
			if err != nil {
				return nil, fmt.Errorf("error checking layer size: %w", err)
			}

			bar = o.Progress.NewBar(diffId.String(), "pull", size)
		}

		existing, err := v.FindVolumeLayer(diffId.String())
		if err != nil {
			return nil, fmt.Errorf("failed to check for existing layer data: %w", err)
		}

		if existing != nil {
			topLayer = existing
			if bar != nil {
				bar.SetCurrent(size)
			}
			continue
		}

		reader, err := layer.Compressed()
		if err != nil {
			return nil, fmt.Errorf("failed getting reader for layer %s: %w", diffId.String(), err)
		}

		if bar != nil {
			reader = bar.ProxyReader(reader)
		}

		reader, err = gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to set up gunzip: %w", err)
		}

		dyn, err := v.NewDynamicLayer("import-"+diffId.String(), WithBackingLayer(topLayer))
		if err != nil {
			return nil, err
		}

		err = dyn.Upload(reader)
		if err != nil {
			_ = dyn.Delete()
			_ = reader.Close()
			return nil, err
		}

		_ = reader.Close()

		vol, err := dyn.ToVolumeLayer(&diffId, opts...)
		if err != nil {
			_ = dyn.Delete()
			return nil, err
		}

		topLayer = vol
	}

	if topLayer == nil {
		return nil, fmt.Errorf("tried importing an empty image")
	}

	return v.MakeImage(name, topLayer)
}

// ImageImportFromReader imports a new image into the local storage pool from a basic reader.
func (v *Virter) ImageImportFromReader(image string, reader io.ReadCloser, opts ...LayerOperationOption) (*LocalImage, error) {
	defer reader.Close()

	diffId := sha256.New()
	teeReader := io.TeeReader(reader, diffId)

	dynLayer, err := v.NewDynamicLayer(image)
	if err != nil {
		return nil, err
	}

	err = dynLayer.Upload(teeReader)
	if err != nil {
		err = fmt.Errorf("failed to transfer data from URL to libvirt: %w", err)
		if rmErr := dynLayer.Delete(); rmErr != nil {
			err = fmt.Errorf("could not remove image: %v, after transfer failed: %w", rmErr, err)
		}
		return nil, err
	}

	hash := regv1.Hash{Algorithm: "sha256", Hex: hex.EncodeToString(diffId.Sum(nil))}
	layer, err := dynLayer.ToVolumeLayer(&hash, opts...)
	if err != nil {
		return nil, err
	}

	return v.MakeImage(image, layer)
}

// ImageRm removes an image from the local storage pool.
//
// If no image of the given name is present, returns nil.
// This will recursively delete any layers that are not referenced by other images or currently in use by a VM.
func (v *Virter) ImageRm(name string) error {
	img, err := v.FindImage(name)
	if err != nil {
		return err
	}

	if img == nil {
		return nil
	}

	return img.tagLayer.DeleteAllIfUnused()
}

// ImageList returns the list of images in the local storage pool.
func (v *Virter) ImageList() ([]*LocalImage, error) {
	vols, _, err := v.libvirt.StoragePoolListAllVolumes(v.provisionStoragePool, -1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list all volumes: %w", err)
	}

	var result []*LocalImage
	for _, vol := range vols {
		if !strings.HasPrefix(vol.Name, TagVolumePrefix) {
			continue
		}

		tagLayer := &RawLayer{
			volume: vol,
			pool:   v.provisionStoragePool,
			conn:   v.libvirt,
		}

		topLayer, err := tagLayer.Dependency()
		if err != nil || topLayer == nil {
			log.WithError(err).Warnf("failed to get top layer for image named %s", vol.Name)
			continue
		}

		result = append(result, &LocalImage{
			tagLayer: tagLayer,
			topLayer: topLayer,
		})
	}

	return result, nil
}

// ImageBuildTools includes the dependencies for building an image
type ImageBuildTools struct {
	ShellClientBuilder ShellClientBuilder
	ContainerProvider  containerapi.ContainerProvider
	AfterNotifier      AfterNotifier
}

// ImageBuildConfig contains the configuration for building an image
type ImageBuildConfig struct {
	ImageName       string
	ContainerName   string
	ShutdownTimeout time.Duration
	ProvisionConfig ProvisionConfig
	ResetMachineID  bool
}

func (v *Virter) imageBuildProvisionCommit(ctx context.Context, tools ImageBuildTools, vmConfig VMConfig, readyConfig VmReadyConfig, buildConfig ImageBuildConfig, user string, opts ...LayerOperationOption) error {
	vmNames := []string{vmConfig.Name}
	var err error

	err = v.WaitVmReady(ctx, tools.ShellClientBuilder, vmConfig.Name, readyConfig, user)
	if err != nil {
		return err
	}

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
			containerCfg := containerapi.NewContainerConfig(
				buildConfig.ContainerName,
				s.Docker.Image,
				s.Docker.Env,
				containerapi.WithCommand(s.Docker.Command...),
				containerapi.WithPullConfig(s.Docker.Pull.ForContainer()),
			)
			err = v.VMExecDocker(ctx, tools.ContainerProvider, vmNames, containerCfg, nil)
		} else if s.Shell != nil {
			err = v.VMExecShell(ctx, vmNames, s.Shell, user)
		} else if s.Rsync != nil {
			copier := netcopy.NewRsyncNetworkCopier()
			err = v.VMExecRsync(ctx, copier, vmNames, s.Rsync)
		}

		if err != nil {
			return err
		}
	}

	err = v.VMCommit(ctx, tools.AfterNotifier, vmConfig.Name, buildConfig.ImageName, true, buildConfig.ShutdownTimeout, vmConfig.StaticDHCP, opts...)
	if err != nil {
		return err
	}

	return nil
}

// ImageBuild builds an image by running a VM and provisioning it.
func (v *Virter) ImageBuild(ctx context.Context, tools ImageBuildTools, vmConfig VMConfig, readyConfig VmReadyConfig, buildConfig ImageBuildConfig, user string, opts ...LayerOperationOption) error {
	// VMRun is responsible to call CheckVMConfig here!
	// TODO(): currently we can not know why VM run failed, so we don't clean up in this stage,
	//         it could have been an existing VM, we don't want to delete it.
	err := v.VMRun(vmConfig)
	if err != nil {
		return err
	}

	// from here on it is safe to rm the VM if something fails
	err = v.imageBuildProvisionCommit(ctx, tools, vmConfig, readyConfig, buildConfig, user, opts...)
	if err != nil {
		log.Warn("could not build image, deleting VM")
		if rmErr := v.VMRm(vmConfig.Name, !vmConfig.StaticDHCP, true); rmErr != nil {
			return fmt.Errorf("could not delete VM: %v, after build failed: %w", rmErr, err)
		}
		return err
	}

	return nil
}
