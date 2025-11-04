package virter_test

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
	"libvirt.org/go/libvirtxml"

	"github.com/LINBIT/virter/internal/virter"
)

func TestVirter_NetworkAdd(t *testing.T) {
	l := newFakeLibvirtConnection()
	v := virter.New(l, poolName, networkName, newMockKeystore())

	err := v.NetworkAdd(libvirtxml.Network{
		Name: "test-net",
	})
	assert.NoError(t, err)

	err = v.NetworkAdd(libvirtxml.Network{
		Name: "test-net",
	})
	assert.Error(t, err)
}

func TestVirter_NetworkList(t *testing.T) {
	l := newFakeLibvirtConnection()
	v := virter.New(l, poolName, networkName, newMockKeystore())

	expectedNets := []libvirtxml.Network{*fakeLibvirtNetwork().description}

	nets, err := v.NetworkList()
	assert.NoError(t, err)
	assert.ElementsMatch(t, expectedNets, nets)

	additionalNet := libvirtxml.Network{
		XMLName: xml.Name{Local: "network"},
		Name:    "test-net",
	}
	err = v.NetworkAdd(additionalNet)
	assert.NoError(t, err)

	nets, err = v.NetworkList()
	assert.NoError(t, err)
	assert.ElementsMatch(t, append(expectedNets, additionalNet), nets)
}

func TestVirter_NetworkRemove(t *testing.T) {
	l := newFakeLibvirtConnection()
	v := virter.New(l, poolName, networkName, newMockKeystore())

	err := v.NetworkRemove(networkName)
	assert.NoError(t, err)
	assert.Empty(t, l.networks)

	err = v.NetworkRemove(networkName)
	assert.NoError(t, err)
	assert.Empty(t, l.networks)
}

func TestVirter_NetworkListAttached(t *testing.T) {
	l := newFakeLibvirtConnection()
	l.addFakeImage(poolName, imageName)
	v := virter.New(l, poolName, networkName, newMockKeystore())
	pool, err := l.StoragePoolLookupByName(poolName)
	assert.NoError(t, err)
	vmnics, err := v.NetworkListAttached(networkName)
	assert.NoError(t, err)
	assert.Empty(t, vmnics)

	img, err := v.FindImage(imageName, pool)
	assert.NoError(t, err)
	assert.NotNil(t, img)

	expected := []virter.VMNic{{VMName: "test", MAC: "52:54:00:00:00:fe"}}
	err = v.VMRun(virter.VMConfig{
		Name:      "test",
		MemoryKiB: 1024,
		VCPUs:     1,
		Image:     img,
	})
	assert.NoError(t, err)
	vmnics, err = v.NetworkListAttached(networkName)
	assert.NoError(t, err)
	assert.Equal(t, expected, vmnics)
}
