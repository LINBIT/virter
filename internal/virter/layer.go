// Work with libvirt storage volumes as if they are simple layers.
//
// This module distinguishes between a RawLayer and a VolumeLayer. The former is a loose wrapper around
// libvirt storage volumes, while the latter is another wrapper around a RawLayer which enforces some additional
// restrictions.

package virter

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/digitalocean/go-libvirt"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	lx "github.com/libvirt/libvirt-go-xml"
	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v7"
)

const (
	// LayerMediaType is the media type used when upload a layer to a container registry.
	LayerMediaType = "application/vnd.com.linbit.virter.layer.v1.qcow2.gzip"
	// LayerVolumePrefix is the prefix for all VolumeLayer storage volumes in libvirt.
	LayerVolumePrefix = "virter:layer:"
	// WorkingLayerNamePrefix is the prefix for all layers that are actively attached to a libvirt VM.
	WorkingLayerNamePrefix = "virter:work:"
)

// RawLayer represents and wraps a libvirt storage volume.
type RawLayer struct {
	conn   LibvirtConnection
	pool   libvirt.StoragePool
	volume libvirt.StorageVol
}

// VolumeLayer is an immutable RawLayer with additional restrictions.
//
// A VolumeLayer is immutable. Since this is not enforceable on the libvirt level, enforcement is best-effort only.
// A VolumeLayer is always addressed by its content, i.e. the backing volume is named
// "virter:layer:sha256:<content-digest>".
type VolumeLayer struct {
	RawLayer
}

// registryVolumeLayer is a gzipped volume layer, compatible with a regv1 Layer.
type registryVolumeLayer struct {
	VolumeLayer
	digest     regv1.Hash
	compressed bytes.Buffer
	opts       layerOperatorOpts
}

type ProgressOpt interface {
	NewBar(name, operation string, total int64) *mpb.Bar
}

type layerOperatorOpts struct {
	Progress ProgressOpt
}
type LayerOperationOption = func(o *layerOperatorOpts)

func WithProgress(p ProgressOpt) LayerOperationOption {
	return func(o *layerOperatorOpts) {
		o.Progress = p
	}
}

func makeLayerOperationOpts(opts ...LayerOperationOption) *layerOperatorOpts {
	o := &layerOperatorOpts{}
	for _, f := range opts {
		f(o)
	}

	return o
}

// FindRawLayer searches the default storage pool for the given volume and returns a RawLayer.
//
// If the layer could not be found, will return (nil, nil).
func (v *Virter) FindRawLayer(name string) (*RawLayer, error) {
	vol, err := v.libvirt.StorageVolLookupByName(v.provisionStoragePool, name)
	if err != nil {
		if hasErrorCode(err, libvirt.ErrNoStorageVol) {
			return nil, nil
		}

		return nil, fmt.Errorf("could not lookup storage volume '%s': %w", name, err)
	}

	return &RawLayer{
		conn:   v.libvirt,
		volume: vol,
		pool:   v.provisionStoragePool,
	}, nil
}

// FindVolumeLayer searches the default storage pool for the given volume layer and returns a VolumeLayer.
//
// The name is the raw layer name, i.e. "sha256:xxxx". If no matching volume is found, returns (nil, nil).
func (v *Virter) FindVolumeLayer(name string) (*VolumeLayer, error) {
	vol, err := v.FindRawLayer(LayerVolumePrefix + name)
	if err != nil {
		return nil, err
	}

	if vol == nil {
		return nil, nil
	}

	return &VolumeLayer{RawLayer: *vol}, nil
}

// NewLayerOption can be passed when creating a new layer to make changes on the libvirt storage volume object.
type NewLayerOption = func(volume *lx.StorageVolume) error

// WithBackingLayer sets the backing layer.
func WithBackingLayer(layer *VolumeLayer) NewLayerOption {
	return func(volume *lx.StorageVolume) error {
		if layer == nil {
			return nil
		}

		volume.BackingStore = layer.asBackingStore()
		desc, err := layer.Descriptor()
		if err != nil {
			return err
		}

		if desc.Capacity != nil {
			if desc.Capacity.Unit != "bytes" {
				return fmt.Errorf("backing volume capacity in '%s' instead of expected 'bytes'", desc.Capacity.Unit)
			}

			// NB: We _always_ use bytes, so comparing the values is fine.
			if volume.Capacity.Value < desc.Capacity.Value {
				volume.Capacity.Value = desc.Capacity.Value
				volume.Capacity.Unit = "bytes"
			}
		}

		return nil
	}
}

// WithCapacity sets the minimal capacity of the new layer in KibiByte.
func WithCapacity(minCapKib uint64) NewLayerOption {
	return func(volume *lx.StorageVolume) error {
		// NB: We _always_ use bytes, so comparing the values is fine.
		if volume.Capacity.Value < minCapKib*1024 {
			volume.Capacity.Value = minCapKib * 1024
			volume.Capacity.Unit = "bytes"
		}

		return nil
	}
}

// WithFormat sets the format used by the storage volume.
//
// Supported values are "qcow2" and "raw".
func WithFormat(fmt string) NewLayerOption {
	return func(volume *lx.StorageVolume) error {
		volume.Target.Format = &lx.StorageVolumeTargetFormat{Type: fmt}
		return nil
	}
}

// NewDynamicLayer create a new (empty) layer suitable for attaching to a VM.
//
// The volume name is prefix with WorkingLayerNamePrefix.
// To set a backing layer, set the minimum capacity and more, pass in one or more NewLayerOption.
func (v *Virter) NewDynamicLayer(name string, opts ...NewLayerOption) (*RawLayer, error) {
	vol, err := v.emptyVolume(DynamicLayerName(name), opts...)
	if err != nil {
		return nil, err
	}

	return &RawLayer{conn: v.libvirt, volume: vol, pool: v.provisionStoragePool}, nil
}

// FindDynamicLayer searches the default storage pool for the given dynamic layer, suitable for attaching to a VM.
//
// If no matching layer could be found, returns (nil, nil).
func (v *Virter) FindDynamicLayer(name string) (*RawLayer, error) {
	return v.FindRawLayer(DynamicLayerName(name))
}

// DynamicLayerName returns the prefixed raw volume name for the given dynamic layer.
func DynamicLayerName(name string) string {
	return WorkingLayerNamePrefix + name
}

// LayerList returns all known VolumeLayer in the default storage pool
func (v *Virter) LayerList() ([]*VolumeLayer, error) {
	vols, _, err := v.libvirt.StoragePoolListAllVolumes(v.provisionStoragePool, -1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list all volumes in storage pool: %w", err)
	}

	var result []*VolumeLayer
	for i := range vols {
		if !strings.HasPrefix(vols[i].Name, LayerVolumePrefix) {
			continue
		}

		result = append(result, &VolumeLayer{
			RawLayer: RawLayer{
				volume: vols[i],
				conn:   v.libvirt,
				pool:   v.provisionStoragePool,
			},
		})
	}

	return result, nil
}

// Name returns the raw volume name
func (rl *RawLayer) Name() string {
	return rl.volume.Name
}

// Upload copies the content of the given reader to the storage volume
func (rl *RawLayer) Upload(reader io.Reader) error {
	err := rl.conn.StorageVolUpload(rl.volume, reader, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to upload to volume: %w", err)
	}

	return nil
}

// Uncompressed returns a reader that yields the raw volume.
//
// If the RawLayer is in qcow2 format, this will read the raw qcow2 file, without any backing store.
func (rl *RawLayer) Uncompressed() (io.ReadCloser, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create pipe for reading volume '%s': %w", rl.volume.Name, err)
	}

	go func() {
		defer writer.Close()

		err := rl.conn.StorageVolDownload(rl.volume, writer, 0, 0, 0)
		if err != nil {
			log.WithError(err).Warnf("failed to copy volume '%s' to pipe", rl.volume.Name)
		}
	}()

	return reader, nil
}

type hashWithProgress struct {
	hash.Hash
	bar *mpb.Bar
}

func (h *hashWithProgress) Write(b []byte) (int, error) {
	n, err := h.Hash.Write(b)
	h.bar.IncrBy(n)
	return n, err
}

// diffIDWithOpts compuates the digest of the Uncompressed layer content
func (rl *RawLayer) diffIDWithOpts(opts ...LayerOperationOption) (regv1.Hash, error) {
	o := makeLayerOperationOpts(opts...)
	hasher := sha256.New()

	if o.Progress != nil {
		desc, err := rl.Descriptor()
		if err != nil {
			return regv1.Hash{}, err
		}

		bar := o.Progress.NewBar(rl.volume.Name, "compute digest", int64(desc.Physical.Value))
		hasher = &hashWithProgress{
			Hash: hasher,
			bar:  bar,
		}
	}

	err := rl.conn.StorageVolDownload(rl.volume, hasher, 0, 0, 0)
	if err != nil {
		return regv1.Hash{}, fmt.Errorf("failed to read volume '%s': %w", rl.volume.Name, err)
	}

	sum := hasher.Sum(nil)
	return regv1.Hash{Algorithm: "sha256", Hex: hex.EncodeToString(sum)}, nil
}

// DiffID computes a digest of the Uncompressed layer content.
func (rl *RawLayer) DiffID() (regv1.Hash, error) {
	return rl.diffIDWithOpts()
}

// Dependency returns a reference to a backing VolumeLayer, if any.
//
// Returns nil if no backing layer was found.
// Returns an error if the backing layer is not a VolumeLayer.
func (rl *RawLayer) Dependency() (*VolumeLayer, error) {
	desc, err := rl.Descriptor()
	if err != nil {
		return nil, err
	}

	if desc.BackingStore == nil {
		return nil, nil
	}

	path := desc.BackingStore.Path
	name := filepath.Base(path)
	key := filepath.Join(filepath.Dir(rl.volume.Key), path)
	storageVol := libvirt.StorageVol{
		Name: name,
		Key:  key,
		Pool: rl.volume.Pool,
	}

	if !strings.HasPrefix(name, LayerVolumePrefix) {
		return nil, &nonVirterVolumeError{name: storageVol.Name}
	}

	return &VolumeLayer{
		RawLayer: RawLayer{
			conn:   rl.conn,
			volume: storageVol,
			pool:   rl.pool,
		},
	}, nil
}

// ToVolumeLayer converts this layer into a VolumeLayer.
//
// First, computes the DiffID of the given layer, creates a new VolumeLayer compatible volume and copies the content
// of this layer over to the new volume. Afterwards, deletes this layer in the libvirt backend.
// If diffID is nil, it will be computed from the volume content.
func (rl *RawLayer) ToVolumeLayer(diffID *regv1.Hash, opts ...LayerOperationOption) (*VolumeLayer, error) {
	if diffID == nil {
		d, err := rl.diffIDWithOpts(opts...)
		if err != nil {
			return nil, err
		}
		diffID = &d
	}

	importName := LayerVolumePrefix + diffID.String()

	importedVolume, err := rl.conn.StorageVolLookupByName(rl.pool, importName)
	if hasErrorCode(err, libvirt.ErrNoStorageVol) {
		clone, err := rl.CloneAs(importName, opts...)
		if err != nil {
			return nil, err
		}

		importedVolume = clone.volume
	} else {
		log.WithField("layer", diffID.String()).Trace("layer already exists")
	}

	vl := &VolumeLayer{RawLayer: RawLayer{volume: importedVolume, conn: rl.conn, pool: rl.pool}}

	err = rl.Delete()
	if err != nil {
		cleanupErr := vl.Delete()
		if cleanupErr != nil {
			log.WithError(cleanupErr).Warnf("could not clean up cloned volume '%s' after failure", importedVolume.Name)
		}
		return nil, fmt.Errorf("failed to remove old volume '%s' after import: %w", rl.volume.Name, err)
	}

	return vl, nil
}

// CloneAs creates a copy of this layer under the given name.
func (rl *RawLayer) CloneAs(name string, opts ...LayerOperationOption) (*RawLayer, error) {
	o := makeLayerOperationOpts(opts...)

	original, err := rl.Descriptor()
	if err != nil {
		return nil, err
	}

	importedVol := lx.StorageVolume{
		Name: name,
		Target: &lx.StorageVolumeTarget{
			Format: &lx.StorageVolumeTargetFormat{Type: "qcow2"},
		},
		Capacity:     &lx.StorageVolumeSize{Value: 0, Unit: "bytes"},
		BackingStore: original.BackingStore,
	}

	encoded, err := importedVol.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to encode copied volume: %w", err)
	}

	clonedVol, err := rl.conn.StorageVolCreateXML(rl.pool, encoded, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to create clone volume: %w", err)
	}

	clonedLayer := &RawLayer{volume: clonedVol, conn: rl.conn, pool: rl.pool}

	// NOTE: we have to use upload/download here, as directly cloning does a rebase for unknown reasons...
	r, err := rl.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get reader for original volume: %w", err)
	}
	if o.Progress != nil {
		bar := o.Progress.NewBar(name, "buffer layer", int64(original.Physical.Value))
		r = bar.ProxyReader(r)
	}

	// NB: evidence collected on my (mwanzenboeck) machine suggests a potential deadlock when using StorageVolDownload
	// (called in Uncompressed()) and StorageVolUpload (called in Upload()). When the amount piped between download and
	// upload is large enough, libvirt seems to get stuck. As a workaround, we first download the layer to a temporary
	// file and upload it only after the download has finished.
	bufferFile, err := ioutil.TempFile("", name)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary clone buffer file: %w", err)
	}

	defer func() {
		_ = bufferFile.Close()
		_ = os.Remove(bufferFile.Name())
	}()

	_, err = io.Copy(bufferFile, r)
	if err != nil {
		return nil, fmt.Errorf("failed to copy layer to temporary clone buffer: %w", err)
	}

	_, err = bufferFile.Seek(0, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to reset temporary clone buffer to start: %w", err)
	}

	// Casting to io.ReaderCloser, so that we can assign ProxyReader if progress is configured.
	var uploadSource io.ReadCloser = bufferFile

	if o.Progress != nil {
		bar := o.Progress.NewBar(name, "upload layer", int64(original.Physical.Value))
		uploadSource = bar.ProxyReader(uploadSource)
	}

	err = clonedLayer.Upload(uploadSource)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to clone volume: %w", err)
	}

	return clonedLayer, nil
}

// Delete deletes this layer.
//
// There are no checks that prevent deletion of an in-use layer (either directly attached, or part of a layer chain).
// A layer that did not exist in the backend will still delete successfully.
func (rl *RawLayer) Delete() error {
	if rl == nil {
		return nil
	}

	err := rl.conn.StorageVolDelete(rl.volume, 0)
	if err != nil {
		if hasErrorCode(err, libvirt.ErrNoStorageVol) {
			return nil
		}

		return fmt.Errorf("failed to remove volume '%s': %w", rl.volume.Name, err)
	}

	return nil
}

func (rl *RawLayer) DeleteAllIfUnused() error {
	if rl == nil {
		return nil
	}

	for {
		dep, err := rl.Dependency()
		if err != nil {
			return err
		}

		deleted, err := rl.DeleteIfUnused()
		if err != nil {
			return err
		}

		if !deleted {
			return nil
		}

		log.WithField("layer", rl.volume.Name).Info("deleted layer")

		if dep == nil {
			return nil
		}

		rl = &dep.RawLayer
	}
}

// DeleteIfUnused deletes the layer if it not referenced as a backing layer of other layers.
//
// This method assumes that only VolumeLayer can have child layers, i.e. if a non VolumeLayer layer depends on another
// non VolumeLayer layer, it will still be deleted. This should not be an issue, as only the top-most layers should be
// mutable (i.e. a non VolumeLayer layer).
func (rl *RawLayer) DeleteIfUnused() (bool, error) {
	vols, _, err := rl.conn.StoragePoolListAllVolumes(rl.pool, -1, 0)
	if err != nil {
		return false, fmt.Errorf("failed to list all volumes: %w", err)
	}

	for i := range vols {
		layer := &RawLayer{pool: rl.pool, conn: rl.conn, volume: vols[i]}

		dep, err := layer.Dependency()
		if err != nil {
			_, isNonVirter := err.(*nonVirterVolumeError)
			if isNonVirter {
				// Not a virter format volume -> shouldn't depend on our volumes.
				continue
			}

			if hasErrorCode(err, libvirt.ErrNoStorageVol) {
				// Volume already deleted -> can't have a relevant dependency anyways.
				continue
			}

			return false, err
		}

		if dep == nil {
			continue
		}

		if dep.RawLayer.volume.Name == rl.volume.Name {
			log.WithFields(log.Fields{"layer": rl, "child": dep}).Trace("layer has dependants, not deleting")
			return false, nil
		}
	}

	err = rl.Delete()
	if err != nil {
		return false, err
	}

	return true, err
}

// Descriptor returns the libvirt descriptor of the storage volume backing this layer.
func (rl *RawLayer) Descriptor() (lx.StorageVolume, error) {
	encoded, err := rl.conn.StorageVolGetXMLDesc(rl.volume, 0)
	if err != nil {
		return lx.StorageVolume{}, fmt.Errorf("failed to get volume descriptor for '%s': %w", rl.volume.Name, err)
	}

	var result lx.StorageVolume
	err = result.Unmarshal(encoded)
	if err != nil {
		return lx.StorageVolume{}, fmt.Errorf("failed to decode volume descriptor for '%s': %w", rl.volume.Name, err)
	}

	return result, nil
}

// ToVolumeLayer converts this layer into a VolumeLayer.
//
// Since VolumeLayer are immutable, this is a no-op.
func (vl *VolumeLayer) ToVolumeLayer() (*VolumeLayer, error) {
	return vl, nil
}

// DiffID computes a digest of the Uncompressed layer content.
//
// Since a VolumeLayer is immutable and named after its content digest, this doesn't do any computation. Instead
// it uses its own name to determine the digest.
func (vl *VolumeLayer) DiffID() (regv1.Hash, error) {
	decoded, err := regv1.NewHash(strings.TrimPrefix(vl.volume.Name, LayerVolumePrefix))
	if err != nil {
		return regv1.Hash{}, &nonVirterVolumeError{name: vl.volume.Name}
	}

	return decoded, nil
}

// Upload copies the content of the given reader to the storage volume
//
// Since a VolumeLayer is immutable, this always results in an error.
func (vl *VolumeLayer) Upload(io.Reader) error {
	return &isImmutableError{vol: vl.volume}
}

// ToRegistryLayer wraps this VolumeLayer for pushing to a container registry.
//
// A layer in a container registry is a compressed (gzipped) blob. To push a VolumeLayer to a registry, we need
// 1. the digest of the _uncompressed_ layer content
// 2. the compressed layer content
// 3. the digest of the _compressed_ layer content
// 4. the size of the _compressed_ layer content
// Points 2-4 are easily computed based on the uncompressed content. For efficiency we only compute it once, as the
// layers can be quite large.
func (vl *VolumeLayer) ToRegistryLayer(opts ...LayerOperationOption) (regv1.Layer, error) {
	o := layerOperatorOpts{}
	for _, f := range opts {
		f(&o)
	}

	reader, err := vl.Uncompressed()
	if err != nil {
		return nil, err
	}

	defer reader.Close()

	if o.Progress != nil {
		desc, err := vl.Descriptor()
		if err != nil {
			return nil, err
		}

		diff, err := vl.DiffID()
		if err != nil {
			return nil, err
		}

		// Assumes bytes (stored volumes always normalize to bytes)
		bar := o.Progress.NewBar(diff.String(), "compress", int64(desc.Physical.Value))
		if bar != nil {
			reader = bar.ProxyReader(reader)
		}
	}

	buf := bytes.Buffer{}
	hasher := sha256.New()

	writer := io.MultiWriter(&buf, hasher)

	compressed, err := gzip.NewWriterLevel(writer, gzip.BestSpeed)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip compressor: %w", err)
	}

	_, err = io.Copy(compressed, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to write compressed temporary layer: %w", err)
	}

	err = compressed.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to flush compressed temporary layer: %w", err)
	}

	hash := hex.EncodeToString(hasher.Sum(nil))

	return &registryVolumeLayer{
		VolumeLayer: *vl,
		compressed:  buf,
		digest:      regv1.Hash{Algorithm: "sha256", Hex: hash},
		opts:        o,
	}, nil
}

// Digest returns the Hash of the compressed layer.
func (vl *registryVolumeLayer) Digest() (regv1.Hash, error) {
	return vl.digest, nil
}

// Compressed returns an io.ReadCloser for the compressed layer contents.
func (vl *registryVolumeLayer) Compressed() (io.ReadCloser, error) {
	reader := ioutil.NopCloser(bytes.NewReader(vl.compressed.Bytes()))

	if vl.opts.Progress != nil {
		diff, err := vl.DiffID()
		if err != nil {
			return nil, err
		}

		bar := vl.opts.Progress.NewBar(diff.String(), "push", int64(vl.compressed.Len()))
		if bar != nil {
			reader = bar.ProxyReader(reader)
		}
	}

	return reader, nil
}

// Size returns the compressed size of the Layer.
func (vl *registryVolumeLayer) Size() (int64, error) {
	return int64(vl.compressed.Len()), nil
}

// MediaType returns the media type for the compressed Layer
func (vl *VolumeLayer) MediaType() (types.MediaType, error) {
	return LayerMediaType, nil
}

// asBackingStore returns this layer as a backing store configuration for a libvirt volume.
func (vl *VolumeLayer) asBackingStore() *lx.StorageVolumeBackingStore {
	if vl == nil {
		return nil
	}

	return &lx.StorageVolumeBackingStore{
		Format: &lx.StorageVolumeTargetFormat{Type: "qcow2"},
		// NB: That we use relative paths here is pretty important. This path is stored in the newly created qcow2
		// file. Absolute paths are bad, as they would also be visible after pushing and pulling the image, even on
		// hosts with different storage pool location. On the other hand, not using "./" would lead qemu to interpret
		// the "virter:" prefix as a protocol used to access the backing volume, which is equally bad.
		Path: "./" + vl.volume.Name,
	}
}

// Squashed creates a squashed copy of this layer.
//
// All backing layers are squashed into a single layer, the returned layer will not depend on any other layers.
func (vl *VolumeLayer) Squashed() (*RawLayer, error) {
	original, err := vl.Descriptor()
	if err != nil {
		return nil, err
	}

	newDesc := lx.StorageVolume{
		Name:     DynamicLayerName("squash-from-" + original.Name),
		Capacity: original.Capacity,
		Target:   &lx.StorageVolumeTarget{Format: &lx.StorageVolumeTargetFormat{Type: "qcow2"}},
	}

	encoded, err := newDesc.Marshal()
	if err != nil {
		return nil, fmt.Errorf("could not encode squashed storage volume: %w", err)
	}

	copyVol, err := vl.conn.StorageVolCreateXMLFrom(vl.pool, encoded, vl.volume, 0)
	if err != nil {
		return nil, fmt.Errorf("could not clone volume: %w", err)
	}

	return &RawLayer{
		volume: copyVol,
		pool:   vl.pool,
		conn:   vl.conn,
	}, nil
}

// emptyVolume creates a new empty volume with the given name and options.
func (v *Virter) emptyVolume(rawname string, opts ...NewLayerOption) (libvirt.StorageVol, error) {
	volumeDescriptor := &lx.StorageVolume{
		Name:     rawname,
		Capacity: &lx.StorageVolumeSize{Value: 0, Unit: "bytes"},
		Target: &lx.StorageVolumeTarget{
			Format: &lx.StorageVolumeTargetFormat{Type: "qcow2"},
		},
	}

	for i := range opts {
		err := opts[i](volumeDescriptor)
		if err != nil {
			return libvirt.StorageVol{}, err
		}
	}

	encoded, err := volumeDescriptor.Marshal()
	if err != nil {
		return libvirt.StorageVol{}, fmt.Errorf("failed to encode volume descriptor for '%s': %w", rawname, err)
	}

	newVol, err := v.libvirt.StorageVolCreateXML(v.provisionStoragePool, encoded, 0)
	if err != nil {
		return libvirt.StorageVol{}, fmt.Errorf("failed to create new volume '%s': %w", rawname, err)
	}

	return newVol, nil
}

// isImmutableError is returned when trying to modify a VolumeLayer
type isImmutableError struct {
	vol libvirt.StorageVol
}

func (i *isImmutableError) Error() string {
	return fmt.Sprintf("'%s' is an immutable volume layer", i.vol.Name)
}

// nonVirterVolumeError is returned when a volume from the storage backend is not in the format expected by Virter.
type nonVirterVolumeError struct {
	name string
}

func (n *nonVirterVolumeError) Error() string {
	return fmt.Sprintf("'%s' is not a virter volume", n.name)
}
