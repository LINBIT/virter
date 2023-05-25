package virter_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"testing/iotest"

	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"

	"github.com/LINBIT/virter/internal/virter"
)

func TestLocalImage_Layers(t *testing.T) {
	l, layer := prepareVolumeLayer(t)
	v := virter.New(l, poolName, networkName, newMockKeystore())

	expectedLayerDiff, err := layer.DiffID()
	assert.NoError(t, err)

	img, err := v.MakeImage("image1", layer)
	assert.NoError(t, err)
	assert.NotNil(t, img)

	layers, err := img.Layers()
	assert.NoError(t, err)
	assert.Len(t, layers, 1)

	actualLayer := layers[0]
	diff, err := actualLayer.DiffID()
	assert.NoError(t, err)
	assert.Equal(t, expectedLayerDiff, diff)
}

func TestVirter_MakeImage(t *testing.T) {
	l, layer := prepareVolumeLayer(t)
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)

	img, err := v.MakeImage("image1", layer)
	assert.NoError(t, err)
	assert.NotNil(t, img)
	assert.Equal(t, "image1", img.Name())
	assert.Equal(t, layer.Name(), img.TopLayer().Name())

	imgFound, err := v.FindImage("image1", pool)
	assert.NoError(t, err)
	assert.NotNil(t, imgFound)
	assert.Equal(t, "image1", imgFound.Name())
	assert.Equal(t, layer.Name(), imgFound.TopLayer().Name())

	imgTagAgain, err := v.MakeImage("image1", layer)
	assert.NoError(t, err)
	assert.NotNil(t, imgTagAgain)
	assert.Equal(t, "image1", imgTagAgain.Name())
	assert.Equal(t, layer.Name(), imgTagAgain.TopLayer().Name())
}

func TestVirter_ImageRm(t *testing.T) {
	l, layer := prepareVolumeLayer(t)
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)

	img, err := v.MakeImage("image1", layer)
	assert.NoError(t, err)
	assert.NotNil(t, img)

	// Unknown images should work
	err = v.ImageRm("image2", pool)
	assert.NoError(t, err)

	err = v.ImageRm("image1", pool)
	assert.NoError(t, err)

	// Should have removed all volumes
	assert.Empty(t, l.pools[poolName].vols)
}

func TestVirter_ImageImportFromReader(t *testing.T) {
	l, layer := prepareVolumeLayer(t)
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)

	img, err := v.ImageImportFromReader("image1", ioutil.NopCloser(strings.NewReader(ExampleLayerContent)), pool)
	assert.NoError(t, err)
	assert.NotNil(t, img)

	assert.Equal(t, img.TopLayer(), layer)
	// 2 volumes: existing content + tag volume
	assert.Len(t, l.pools[poolName].vols, 2)

	img2, err := v.ImageImportFromReader("image2", ioutil.NopCloser(strings.NewReader(ExampleLayerContent+ExampleLayerContent)), pool)
	assert.NoError(t, err)
	assert.NotNil(t, img2)

	assert.NotEqual(t, img2.TopLayer(), layer)
	// 4 volumes: 2 * (existing content + tag volume)
	assert.Len(t, l.pools[poolName].vols, 4)

	failed, err := v.ImageImportFromReader("image2", ioutil.NopCloser(iotest.ErrReader(errors.New("test"))), pool)
	assert.Error(t, err)
	assert.Nil(t, failed)
	// 4 volumes: 2 * (existing content + tag volume), no new volume from failed import
	assert.Len(t, l.pools[poolName].vols, 4)
}

func TestVirter_ImageList(t *testing.T) {
	l := newFakeLibvirtConnection()
	l.addFakeImage(poolName, "image1")
	l.addFakeImage(poolName, "image2")
	v := virter.New(l, poolName, networkName, newMockKeystore())

	imgs, err := v.ImageList()
	assert.NoError(t, err)
	assert.Len(t, imgs, 2)

	var names []string
	for _, img := range imgs {
		names = append(names, img.Name())
	}
	assert.ElementsMatch(t, []string{"image1", "image2"}, names)

	assert.Equal(t, imgs[0].TopLayer(), imgs[1].TopLayer())
}

type inMemoryLayer struct {
	content string
}

func (i *inMemoryLayer) DiffID() (regv1.Hash, error) {
	hasher := sha256.New()
	_, err := hasher.Write([]byte(i.content))
	if err != nil {
		return regv1.Hash{}, err
	}

	return regv1.Hash{Algorithm: "sha256", Hex: hex.EncodeToString(hasher.Sum(nil))}, nil
}

func (i *inMemoryLayer) Uncompressed() (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader(i.content)), nil
}

func (i *inMemoryLayer) MediaType() (types.MediaType, error) {
	return virter.LayerMediaType, nil
}

func fakeRemoteLayer(t *testing.T, content string) regv1.Layer {
	layer, err := partial.UncompressedToLayer(&inMemoryLayer{
		content: content,
	})
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	return layer
}

func TestVirter_ImageImport(t *testing.T) {
	l, base := prepareVolumeLayer(t)
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)

	source := &fake.FakeImage{
		ManifestStub: func() (*regv1.Manifest, error) {
			return &regv1.Manifest{
				Config: regv1.Descriptor{
					MediaType: virter.ImageMediaType,
				},
			}, nil
		},
		LayersStub: func() ([]regv1.Layer, error) {
			return []regv1.Layer{
				fakeRemoteLayer(t, ExampleLayerContent),
				fakeRemoteLayer(t, "some more content"),
			}, nil
		},
	}

	local, err := v.ImageImport("name", pool, source)
	assert.NoError(t, err)
	assert.NotNil(t, local)

	assert.Equal(t, "name", local.Name())
	// sha256("some more content")
	assert.Equal(t, virter.LayerVolumePrefix+"sha256:c13faca63307342e622347733e82496954c9d56a0c5b90af6e0fb7aa7e920ad2", local.TopLayer().Name())

	actualbase, err := local.TopLayer().Dependency()
	assert.NoError(t, err)
	assert.NotNil(t, actualbase)
	assert.Equal(t, base.Name(), actualbase.Name())
	assert.Contains(t, l.pools[poolName].vols, virter.TagVolumePrefix+"name")
	assert.Contains(t, l.pools[poolName].vols, virter.LayerVolumePrefix+"sha256:c13faca63307342e622347733e82496954c9d56a0c5b90af6e0fb7aa7e920ad2")
	assert.Contains(t, l.pools[poolName].vols, virter.LayerVolumePrefix+ExampleLayerDigest)
}

func TestVirter_Image(t *testing.T) {
	l := newFakeLibvirtConnection()
	l.addFakeImage(poolName, "image1")
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)

	img, err := v.FindImage("image1", pool)
	assert.NoError(t, err)
	assert.NotNil(t, img)

	mt, err := img.MediaType()
	assert.NoError(t, err)
	assert.Equal(t, types.DockerManifestSchema2, mt)

	cfg, err := img.ConfigName()
	assert.NoError(t, err)
	assert.Equal(t, "sha256:697fac62e6b113ea0d9865703491f88d8d89672ea3d33a35359e0d29f0d0bbd9", cfg.String())

	layer, err := img.LayerByDiffID(regv1.Hash{Algorithm: "sha256", Hex: ExampleLayerDigest[len("sha256:"):]})
	assert.NoError(t, err)
	assert.NotNil(t, layer)

	reader, err := layer.Uncompressed()
	assert.NoError(t, err)

	layerContent, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, ExampleLayerContent, string(layerContent))
}
