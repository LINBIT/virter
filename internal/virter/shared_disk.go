package virter

import (
	"fmt"
	"sort"
	"strings"
)

// SharedDiskPrefix is the prefix for all shared disk volumes in libvirt.
const SharedDiskPrefix = "virter:shared:"

// SharedDiskName returns the volume name for the shared disk with the given name.
func SharedDiskName(name string) string {
	return SharedDiskPrefix + name
}

// SharedDisk describes a disk volume that can be attached to multiple VMs at the same time.
type SharedDisk struct {
	Name       string
	SizeB      uint64
	AttachedTo []string
}

// SharedDiskCreate creates a raw volume that can be attached to multiple VMs at the same time.
//
// The disk is not attached to any VM. Use the "--shared-disk" option when starting a VM to attach it.
func (v *Virter) SharedDiskCreate(name string, poolName string, sizeKiB uint64) error {
	if sizeKiB == 0 {
		return fmt.Errorf("cannot create shared disk '%s' with size 0", name)
	}

	pool, err := v.lookupPool(poolName)
	if err != nil {
		return fmt.Errorf("failed to lookup libvirt pool '%s': %w", poolName, err)
	}

	existing, err := v.FindRawLayer(SharedDiskName(name), pool)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("shared disk '%s' already exists", name)
	}

	// Shared disks are always raw: qcow2 metadata cannot be safely written to by multiple VMs.
	_, err = v.emptyVolume(SharedDiskName(name), pool, WithCapacity(sizeKiB), WithFormat("raw"))
	if err != nil {
		return fmt.Errorf("failed to create shared disk '%s': %w", name, err)
	}

	return nil
}

// SharedDiskRm removes a shared disk. Removing a disk which is still attached to a VM is an error.
//
// Removing a shared disk that does not exist is not an error.
func (v *Virter) SharedDiskRm(name string, poolName string) error {
	pool, err := v.lookupPool(poolName)
	if err != nil {
		return fmt.Errorf("failed to lookup libvirt pool '%s': %w", poolName, err)
	}

	attached, err := v.domainsUsingVolume(pool.Name, SharedDiskName(name))
	if err != nil {
		return err
	}
	if len(attached) > 0 {
		return fmt.Errorf("shared disk '%s' is still attached to: %s", name, strings.Join(attached, ", "))
	}

	layer, err := v.FindRawLayer(SharedDiskName(name), pool)
	if err != nil {
		return err
	}

	return layer.Delete()
}

// SharedDiskList lists all shared disks in the given pool, including the VMs they are attached to.
func (v *Virter) SharedDiskList(poolName string) ([]SharedDisk, error) {
	pool, err := v.lookupPool(poolName)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup libvirt pool '%s': %w", poolName, err)
	}

	vols, _, err := v.libvirt.StoragePoolListAllVolumes(pool, -1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	var result []SharedDisk
	for _, vol := range vols {
		if !strings.HasPrefix(vol.Name, SharedDiskPrefix) {
			continue
		}

		_, capacity, _, err := v.libvirt.StorageVolGetInfo(vol)
		if err != nil {
			return nil, fmt.Errorf("failed to get info for volume '%s': %w", vol.Name, err)
		}

		attached, err := v.domainsUsingVolume(pool.Name, vol.Name)
		if err != nil {
			return nil, err
		}

		result = append(result, SharedDisk{
			Name:       strings.TrimPrefix(vol.Name, SharedDiskPrefix),
			SizeB:      capacity,
			AttachedTo: attached,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// domainsUsingVolume returns the names of all domains that have the given volume attached.
func (v *Virter) domainsUsingVolume(poolName string, volumeName string) ([]string, error) {
	domains, _, err := v.libvirt.ConnectListAllDomains(-1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}

	var result []string
	for _, domain := range domains {
		disks, err := v.getDisksOfDomain(domain)
		if err != nil {
			return nil, err
		}

		for _, disk := range disks {
			if disk.poolName == poolName && disk.volumeName == volumeName {
				result = append(result, domain.Name)
				break
			}
		}
	}

	return result, nil
}
