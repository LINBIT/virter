package virter_test

import (
	"testing"

	"github.com/digitalocean/go-libvirt"
	"github.com/stretchr/testify/assert"

	"github.com/LINBIT/virter/internal/virter"
)

func TestSharedDiskLifecycle(t *testing.T) {
	l := newFakeLibvirtConnection()
	v := virter.New(l, poolName, networkName, newMockKeystore())

	err := v.SharedDiskCreate(sharedDiskName, "", 10*1024)
	assert.NoError(t, err)

	err = v.SharedDiskCreate(sharedDiskName, "", 10*1024)
	assert.Error(t, err, "creating an existing disk should fail")

	disks, err := v.SharedDiskList("")
	assert.NoError(t, err)
	if assert.Len(t, disks, 1) {
		assert.Equal(t, sharedDiskName, disks[0].Name)
		assert.Equal(t, uint64(10*1024*1024), disks[0].SizeB)
		assert.Empty(t, disks[0].AttachedTo)
	}

	err = v.SharedDiskRm(sharedDiskName, "")
	assert.NoError(t, err)

	disks, err = v.SharedDiskList("")
	assert.NoError(t, err)
	assert.Empty(t, disks)

	err = v.SharedDiskRm(sharedDiskName, "")
	assert.NoError(t, err, "removing a missing disk should succeed")
}

// staleDomainConnection lists an extra domain that no longer exists,
// simulating a domain removed after ConnectListAllDomains returned.
type staleDomainConnection struct {
	*FakeLibvirtConnection
	staleName string
}

func (c *staleDomainConnection) ConnectListAllDomains(needResults int32, flags libvirt.ConnectListAllDomainsFlags) ([]libvirt.Domain, uint32, error) {
	domains, count, err := c.FakeLibvirtConnection.ConnectListAllDomains(needResults, flags)
	domains = append(domains, libvirt.Domain{Name: c.staleName})
	return domains, count + 1, err
}

func TestSharedDiskRmIgnoresRemovedDomain(t *testing.T) {
	l := &staleDomainConnection{
		FakeLibvirtConnection: newFakeLibvirtConnection(),
		staleName:             "removed-domain",
	}
	v := virter.New(l, poolName, networkName, newMockKeystore())

	err := v.SharedDiskCreate(sharedDiskName, "", 10*1024)
	assert.NoError(t, err)

	err = v.SharedDiskRm(sharedDiskName, "")
	assert.NoError(t, err, "domains removed while iterating should be ignored")
}

func TestSharedDiskRmAttached(t *testing.T) {
	l := newFakeLibvirtConnection()
	v := virter.New(l, poolName, networkName, newMockKeystore())

	err := v.SharedDiskCreate(sharedDiskName, "", 10*1024)
	assert.NoError(t, err)

	domain := newFakeLibvirtDomain(vmName, vmMAC)
	domain.persistent = true
	domain.active = true
	addSharedDisk(domain, sharedDiskName, poolName, "vdb")
	l.domains[vmName] = domain

	err = v.SharedDiskRm(sharedDiskName, "")
	assert.Error(t, err, "removing an attached disk should fail")

	disks, err := v.SharedDiskList("")
	assert.NoError(t, err)
	if assert.Len(t, disks, 1) {
		assert.Equal(t, []string{vmName}, disks[0].AttachedTo)
	}
}
