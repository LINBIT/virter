package virter_test

import (
	"io"
	"io/ioutil"
	"testing"

	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/stretchr/testify/assert"
	lx "libvirt.org/go/libvirtxml"

	"github.com/LINBIT/virter/internal/virter"
)

func TestVirter_GetRawLayer(t *testing.T) {
	l := newFakeLibvirtConnection()
	l.pools[poolName].vols["exists"] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: "exists"}}
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)

	layer, err := v.FindRawLayer("does not exist", pool)
	assert.NoError(t, err)
	assert.Nil(t, layer)

	layer, err = v.FindRawLayer("exists", pool)
	assert.NoError(t, err)
	assert.NotNil(t, layer)
	assert.Equal(t, "exists", layer.Name())
}

func TestVirter_GetVolumeLayer(t *testing.T) {
	l := newFakeLibvirtConnection()
	l.pools[poolName].vols[virter.LayerVolumePrefix+"exists"] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: virter.LayerVolumePrefix + "exists"}}
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)

	layer, err := v.FindVolumeLayer("does not exist", pool)
	assert.NoError(t, err)
	assert.Nil(t, layer)

	layer, err = v.FindVolumeLayer("exists", pool)
	assert.NoError(t, err)
	assert.NotNil(t, layer)
	assert.Equal(t, virter.LayerVolumePrefix+"exists", layer.Name())
}

func TestVirter_LayerList(t *testing.T) {
	l := newFakeLibvirtConnection()
	l.pools[poolName].vols[virter.LayerVolumePrefix+"exists"] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: virter.LayerVolumePrefix + "exists"}}
	l.pools[poolName].vols[virter.LayerVolumePrefix+"exists2"] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: virter.LayerVolumePrefix + "exists2"}}
	l.pools[poolName].vols["non-layer"] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: "non-layer"}}
	l.pools[poolName].vols[virter.TagVolumePrefix+"tag"] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: virter.TagVolumePrefix + "tag"}}
	v := virter.New(l, poolName, networkName, newMockKeystore())

	layers, err := v.LayerList()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(layers))

	names := make([]string, len(layers))
	for i, layer := range layers {
		names[i] = layer.Name()
	}

	assert.ElementsMatch(t, []string{virter.LayerVolumePrefix + "exists", virter.LayerVolumePrefix + "exists2"}, names)
}

func TestRawLayer_Uncompressed(t *testing.T) {
	l := newFakeLibvirtConnection()
	l.pools[poolName].vols["empty"] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: "empty"}, content: nil}
	l.pools[poolName].vols["content"] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: "content"}, content: []byte(ExampleLayerContent)}
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)

	emptyLayer, err := v.FindRawLayer("empty", pool)
	assert.NoError(t, err)

	emptyReader, err := emptyLayer.Uncompressed()
	assert.NoError(t, err)

	defer emptyReader.Close()

	empty, err := io.ReadAll(emptyReader)
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, empty)

	contentLayer, err := v.FindRawLayer("content", pool)
	assert.NoError(t, err)

	contentReader, err := contentLayer.Uncompressed()
	assert.NoError(t, err)

	defer contentReader.Close()

	content, err := io.ReadAll(contentReader)
	assert.NoError(t, err)
	assert.Equal(t, []byte(ExampleLayerContent), content)
}

func TestRawLayer_ToVolumeLayer(t *testing.T) {
	l := newFakeLibvirtConnection()
	l.pools[poolName].vols["content"] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: "content"}, content: []byte(ExampleLayerContent)}
	l.pools[poolName].vols["content-clone"] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: "content-clone"}, content: []byte(ExampleLayerContent)}
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)

	initLayer, err := v.FindRawLayer("content", pool)
	assert.NoError(t, err)
	assert.NotNil(t, initLayer)

	vl, err := initLayer.ToVolumeLayer(nil)
	assert.NoError(t, err)
	assert.Equal(t, virter.LayerVolumePrefix+ExampleLayerDigest, vl.Name())

	// Check that the layer is deleted after conversion
	initLayerCopy, err := v.FindRawLayer("content", pool)
	assert.NoError(t, err)
	assert.Nil(t, initLayerCopy)

	// Check importing again does nothing
	vlAgain, err := vl.ToVolumeLayer()
	assert.NoError(t, err)
	assert.Equal(t, vlAgain, vl)

	// Check importing volume with same content has same result
	rawClone, err := v.FindRawLayer("content-clone", pool)
	assert.NoError(t, err)
	assert.NotNil(t, rawClone)

	vlClone, err := rawClone.ToVolumeLayer(nil)
	assert.NoError(t, err)
	assert.NotNil(t, vlClone)
	assert.Equal(t, vlClone, vl)

	// Check cloned copy is also removed
	rawCloneAgain, err := v.FindRawLayer("content-clone", pool)
	assert.NoError(t, err)
	assert.Nil(t, rawCloneAgain)
}

func TestRawLayer_DeleteIfUnused(t *testing.T) {
	l, backingLayer := prepareVolumeLayer(t)
	l.pools[poolName].vols["childLayer"] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: "childLayer", BackingStore: &lx.StorageVolumeBackingStore{Path: "./" + backingLayer.Name()}}, content: []byte("override")}
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)

	childLayer, err := v.FindRawLayer("childLayer", pool)
	assert.NoError(t, err)
	assert.NotNil(t, backingLayer)

	deleted, err := backingLayer.DeleteIfUnused()
	assert.NoError(t, err)
	assert.False(t, deleted)

	deleted, err = childLayer.DeleteIfUnused()
	assert.NoError(t, err)
	assert.True(t, deleted)
	assert.Contains(t, l.pools[poolName].vols, backingLayer.Name())
	assert.NotContains(t, l.pools[poolName].vols, "childLayer")

	deleted, err = backingLayer.DeleteIfUnused()
	assert.NoError(t, err)
	assert.True(t, deleted)
	assert.NotContains(t, l.pools[poolName].vols, backingLayer.Name())
}

const (
	ExampleLayerContent = "something to download"
	ExampleLayerDigest  = "sha256:8575b86cb19cdff6a47b8cddc00261c34acc3faea6e120eb6eccbca867c00b4b"
)

func prepareVolumeLayer(t *testing.T) (*FakeLibvirtConnection, *virter.VolumeLayer) {
	l := newFakeLibvirtConnection()
	l.pools[poolName].vols[virter.LayerVolumePrefix+ExampleLayerDigest] = &FakeLibvirtStorageVol{description: &lx.StorageVolume{Name: virter.LayerVolumePrefix + ExampleLayerDigest}, content: []byte(ExampleLayerContent)}
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)

	layer, err := v.FindVolumeLayer(ExampleLayerDigest, pool)
	assert.NoError(t, err)
	assert.NotNil(t, layer)

	return l, layer
}

func TestVolumeLayer_ToVolumeLayer(t *testing.T) {
	_, layer := prepareVolumeLayer(t)

	layerCopy, err := layer.ToVolumeLayer()
	assert.NoError(t, err)
	assert.Equal(t, layer, layerCopy)
}

func TestVolumeLayer_DiffID(t *testing.T) {
	_, layer := prepareVolumeLayer(t)

	diff, err := layer.DiffID()
	assert.NoError(t, err)
	assert.Equal(t, regv1.Hash{Algorithm: "sha256", Hex: "8575b86cb19cdff6a47b8cddc00261c34acc3faea6e120eb6eccbca867c00b4b"}, diff)
}

func TestVolumeLayer_Upload(t *testing.T) {
	_, layer := prepareVolumeLayer(t)

	// Volume layers can't read
	err := layer.Upload(nil)
	assert.Error(t, err)
}

func TestVolumeLayer_ToRegistryLayer(t *testing.T) {
	_, layer := prepareVolumeLayer(t)

	compatLayer, err := layer.ToRegistryLayer()
	assert.NoError(t, err)
	assert.NotNil(t, compatLayer)

	uncompressedReader, err := compatLayer.Uncompressed()
	assert.NoError(t, err)

	actual, err := ioutil.ReadAll(uncompressedReader)
	assert.NoError(t, err)
	assert.Equal(t, ExampleLayerContent, string(actual))

	size, err := compatLayer.Size()
	assert.NoError(t, err)
	// Compression actually makes this bigger since some metadata is added.
	assert.Equal(t, int64(49), size)

	digest, err := compatLayer.Digest()
	assert.NoError(t, err)
	assert.Equal(t, regv1.Hash{Algorithm: "sha256", Hex: "e4556089c0b3b5e611714f04b9cea89c026333ef0cd40d5f6b0658a7cf8ee242"}, digest)

	diff, err := compatLayer.DiffID()
	assert.NoError(t, err)
	assert.Equal(t, regv1.Hash{Algorithm: "sha256", Hex: ExampleLayerDigest[len("sha256:"):]}, diff)
}

func TestVolumeLayer_Squashed(t *testing.T) {
	_, layer := prepareVolumeLayer(t)

	raw, err := layer.Squashed()
	assert.NoError(t, err)

	desc, err := raw.Descriptor()
	assert.NoError(t, err)
	assert.Nil(t, desc.BackingStore)
}
