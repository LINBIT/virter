package libvirtinterfaces

import (
	"github.com/libvirt/libvirt-go"
)

// LibvirtConnect contains required libvirt connection methods.
type LibvirtConnect interface {
	LookupStoragePoolByName(name string) (LibvirtStoragePool, error)
	NewStream(flags libvirt.StreamFlags) (LibvirtStream, error)
}

// LibvirtStoragePool contains required libvirt storage pool methods.
type LibvirtStoragePool interface {
	GetUUIDString() (string, error)
}

// LibvirtStream contains required libvirt stream methods.
type LibvirtStream interface {
}
