package virter_test

import (
	"encoding/xml"
	"github.com/LINBIT/virter/internal/virter"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
	"github.com/stretchr/testify/assert"
	"testing"
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
	l.vols[imageName] = &FakeLibvirtStorageVol{}
	v := virter.New(l, poolName, networkName, newMockKeystore())
	vmnics, err := v.NetworkListAttached(networkName)
	assert.NoError(t, err)
	assert.Empty(t, vmnics)

	expected := []virter.VMNic{{VMName: "test", MAC: "52:54:00:00:00:fe"}}
	err = v.VMRun(virter.VMConfig{
		Name:      "test",
		MemoryKiB: 1024,
		VCPUs:     1,
		ImageName: imageName,
	})
	assert.NoError(t, err)
	vmnics, err = v.NetworkListAttached(networkName)
	assert.NoError(t, err)
	assert.Equal(t, expected, vmnics)
}
