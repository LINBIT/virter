package virter_test

import (
	"testing"

	libvirtxml "github.com/libvirt/libvirt-go-xml"
	"github.com/stretchr/testify/assert"

	"github.com/LINBIT/virter/internal/virter"
)

type fakeNetworkNic string

func (f fakeNetworkNic) GetType() string {
	return "network"
}

func (f fakeNetworkNic) GetSource() string {
	return string(f)
}

func (f fakeNetworkNic) GetModel() string {
	return "virtio"
}

func (f fakeNetworkNic) GetMAC() string {
	return "fake"
}

var testNetworks = map[string][]libvirtxml.NetworkIP{
	"dhcp1":  {{DHCP: &libvirtxml.NetworkDHCP{}}},
	"dhcp2":  {{DHCP: &libvirtxml.NetworkDHCP{}}},
	"nodhcp": {{}},
	"noip":   nil,
}

func TestVirter_NetworkConfig(t *testing.T) {
	l := newFakeLibvirtConnection()
	v := virter.New(l, poolName, networkName, newMockKeystore())

	for name, ips := range testNetworks {
		err := v.NetworkAdd(libvirtxml.Network{Name: name, IPs: ips})
		assert.NoError(t, err)
	}

	testcases := []struct {
		name     string
		nics     []virter.NIC
		expected string
	}{
		{
			name:     "default-no-config",
			expected: "",
		},
		{
			name: "all-dhcp-config",
			nics: []virter.NIC{
				fakeNetworkNic("dhcp1"),
				fakeNetworkNic("dhcp2"),
			},
			expected: `version: 2
ethernets:
  eth0:
    dhcp4: true
  enp1s0:
    dhcp4: true
  eth1:
    dhcp4: true
  enp2s0:
    dhcp4: true
  eth2:
    dhcp4: true
  enp3s0:
    dhcp4: true
`,
		},
		{
			name: "some-without-dhcp-no-config",
			nics: []virter.NIC{
				fakeNetworkNic("dhcp1"),
				fakeNetworkNic("nodhcp"),
			},
			expected: "",
		},
		{
			name: "some-without-ip-no-config",
			nics: []virter.NIC{
				fakeNetworkNic("dhcp1"),
				fakeNetworkNic("noip"),
			},
			expected: "",
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			actual, err := v.NetworkConfig(tcase.nics)
			assert.NoError(t, err)
			assert.Equal(t, tcase.expected, actual)
		})
	}
}
