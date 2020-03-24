package virter_test

import (
	"github.com/digitalocean/go-libvirt"

	"github.com/LINBIT/virter/internal/virter/mocks"
)

// mockLibvirtConnection provides additional mocking functionality specific to
// our requirements. The standard mock objects only allow for fixed return
// values for each mocked call. Occasionally this is not sufficient for us.
type mockLibvirtConnection struct {
	mocks.LibvirtConnection
	overrideIsActive bool
	// isActive is a list of successive return values for DomainIsActive
	isActive []int32
}

// DomainIsActive returns whether the mocked domain is active
func (m *mockLibvirtConnection) DomainIsActive(Dom libvirt.Domain) (int32, error) {
	if m.overrideIsActive {
		var a int32
		a, m.isActive = m.isActive[0], m.isActive[1:]
		return a, nil
	}

	return m.LibvirtConnection.DomainIsActive(Dom)
}
