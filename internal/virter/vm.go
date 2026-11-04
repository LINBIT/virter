package virter

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LINBIT/containerapi"
	sshclient "github.com/LINBIT/gosshclient"
	libvirt "github.com/digitalocean/go-libvirt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	lx "libvirt.org/go/libvirtxml"

	"github.com/LINBIT/virter/pkg/actualtime"
	"github.com/LINBIT/virter/pkg/netcopy"
	"github.com/LINBIT/virter/pkg/sshkeys"
)

func (v *Virter) VMExists(vmName string) error {
	if _, err := v.getMetaForVM(vmName); err != nil {
		return fmt.Errorf("failed to get VM metadata: %w", err)
	}

	return nil
}

// lookupPool looks up a libvirt pool by name, falling back to the default
// storage pool if the name is empty.
func (v *Virter) lookupPool(name string) (libvirt.StoragePool, error) {
	if name == v.provisionStoragePool.Name || name == "" {
		return v.provisionStoragePool, nil
	}

	return v.libvirt.StoragePoolLookupByName(name)
}

func (v *Virter) anyImageExists(vmConfig VMConfig) (bool, error) {
	vmName := vmConfig.Name
	type imageAndPool struct {
		image string
		pool  libvirt.StoragePool
	}
	imgs := []imageAndPool{
		{vmName, v.provisionStoragePool},
		{ciDataVolumeName(vmName), v.provisionStoragePool},
	}

	for _, d := range vmConfig.Disks {
		if d.GetShareable() {
			// shared disks are created by "virter disk create" and are expected to exist
			continue
		}
		pool, err := v.lookupPool(d.GetPool())
		if err != nil {
			return false, fmt.Errorf("failed to lookup libvirt pool: %w", err)
		}
		imgs = append(imgs, imageAndPool{diskVolumeName(vmName, d.GetName()), pool})
	}

	for _, img := range imgs {
		if layer, err := v.FindDynamicLayer(img.image, img.pool); layer != nil || err != nil {
			return layer != nil, err
		}
	}
	return false, nil
}

func (v *Virter) VMList() ([]string, error) {
	domains, _, err := v.libvirt.ConnectListAllDomains(-1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains")
	}

	result := make([]string, len(domains))
	for i, domain := range domains {
		result[i] = domain.Name
	}

	return result, nil
}

type VMInfo struct {
	Name          string
	ID            uint
	AccessNetwork string
	Running       bool
}

// VMInfo returns the ID, access network, and running state of the named VM
func (v *Virter) VMInfo(vmName string) (*VMInfo, error) {
	dom, err := v.libvirt.DomainLookupByName(vmName)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup domain: %w", err)
	}

	active, err := v.libvirt.DomainIsActive(dom)
	if err != nil {
		return nil, fmt.Errorf("failed to check if domain is active: %w", err)
	}

	xmldesc, err := v.libvirt.DomainGetXMLDesc(dom, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get XML description: %w", err)
	}

	desc := lx.Domain{}
	err = xml.Unmarshal([]byte(xmldesc), &desc)
	if err != nil {
		return nil, fmt.Errorf("could not decode domain xml '%s': %w", vmName, err)
	}

	meta := metaWrapper{}
	if desc.Metadata == nil {
		// not a virter VM
		return &VMInfo{Name: vmName, Running: active != 0}, nil
	}
	err = xml.Unmarshal([]byte(desc.Metadata.XML), &meta)
	if err != nil {
		// not a virter VM
		return &VMInfo{Name: vmName, Running: active != 0}, nil
	}

	accessNIC, err := getAccessNIC(vmName, &desc)
	if err != nil {
		return nil, err
	}

	vmMac, err := net.ParseMAC(accessNIC.MAC)
	if err != nil {
		return nil, fmt.Errorf("failed to parse VM MAC address '%s' for VM '%s': %w", accessNIC.MAC, vmName, err)
	}

	return &VMInfo{Name: vmName, AccessNetwork: accessNIC.Network, ID: IDFromMAC(vmMac, QemuBaseMAC()), Running: active != 0}, nil
}

// VMRun starts a VM.
func (v *Virter) VMRun(vmConfig VMConfig) error {
	// checks
	vmConfig, err := CheckVMConfig(vmConfig)
	if err != nil {
		return err
	}

	var machine []string
	if vmConfig.CpuArch.OSDomain().Type.Machine != "" {
		machine = append(machine, vmConfig.CpuArch.OSDomain().Type.Machine)
	}

	domCapsXML, err := v.libvirt.ConnectGetDomainCapabilities(nil, []string{vmConfig.CpuArch.QemuArch()}, machine, nil, 0)
	if err != nil {
		return fmt.Errorf("host does not support emulating %s. install qemu-system-%s", vmConfig.CpuArch, vmConfig.CpuArch.QemuArch())
	}

	if vmConfig.NestedVirtualization {
		feature, err := NestedVirtFeature(domCapsXML)
		if err != nil {
			return err
		}
		vmConfig.nestedVirtFeature = feature
	}

	vmName := vmConfig.Name
	_, err = v.libvirt.DomainLookupByName(vmName)
	if !hasErrorCode(err, libvirt.ErrNoDomain) {
		if err != nil {
			return fmt.Errorf("could not get domain: %w", err)
		}
		return fmt.Errorf("domain '%s' already defined", vmName)
	}

	if exists, err := v.anyImageExists(vmConfig); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("one of the images already exists")
	}

	for _, d := range vmConfig.Disks {
		if !d.GetShareable() {
			continue
		}
		pool, err := v.lookupPool(d.GetPool())
		if err != nil {
			return fmt.Errorf("failed to lookup libvirt pool '%s': %w", d.GetPool(), err)
		}
		layer, err := v.FindRawLayer(SharedDiskName(d.GetName()), pool)
		if err != nil {
			return err
		}
		if layer == nil {
			return fmt.Errorf("shared disk '%s' does not exist, create it with 'virter disk create'", d.GetName())
		}
	}

	// Reclaim a DHCP reservation orphaned by an unclean removal so the ID is reusable
	if !vmConfig.StaticDHCP && vmConfig.ID != 0 {
		if err := v.reclaimOrphanDHCPHost(vmConfig.ID); err != nil {
			return err
		}
	}

	id, err := v.GetVMID(vmConfig.ID, vmConfig.StaticDHCP)
	if err != nil {
		return err
	}
	vmConfig.ID = id

	mac := QemuMAC(vmConfig.ID)

	existingDomain, err := v.getDomainForMAC(mac)
	if err != nil {
		return err
	}
	if existingDomain.Name != "" {
		return fmt.Errorf("MAC address '%s' already in use by domain '%s'", mac, existingDomain.Name)
	}
	// end checks

	log.Debug("Create host key")
	hostkey, err := sshkeys.NewRSAHostKey()
	if err != nil {
		return fmt.Errorf("could not create new host key: %w", err)
	}

	meta := &VMMeta{
		HostKey:            hostkey.PublicKey(),
		SSHUserName:        vmConfig.SSHUserName,
		SSHTrustOnFirstUse: vmConfig.SSHTrustOnFirstUse,
		SkipCloudInitCheck: vmConfig.SkipCloudInitCheck,
	}
	if vmConfig.SSHTrustOnFirstUse {
		meta.HostKey = ""
	}

	vmXML, err := v.vmXML(vmConfig, mac, meta)
	if err != nil {
		return err
	}

	log.Debugf("Using domain XML: %s", vmXML)

	log.Debug("Define VM")
	d, err := v.libvirt.DomainDefineXML(vmXML)
	if err != nil {
		return fmt.Errorf("could not define domain: %w", err)
	}

	log.Debug("Create boot volume")
	_, err = v.ImageSpawn(vmConfig.Name, v.provisionStoragePool, vmConfig.Image, vmConfig.BootCapacityKiB)
	if err != nil {
		return err
	}

	log.Debug("Create cloud-init volume")
	_, err = v.createCIData(vmConfig, hostkey)
	if err != nil {
		return err
	}

	for _, d := range vmConfig.Disks {
		if d.GetShareable() {
			continue
		}
		log.WithField("name", d.GetName()).Debug("Create volume")
		pool, err := v.lookupPool(d.GetPool())
		if err != nil {
			return fmt.Errorf("failed to lookup libvirt pool %s: %w", d.GetPool(), err)
		}
		_, err = v.NewDynamicLayer(diskVolumeName(vmConfig.Name, d.GetName()), pool, WithCapacity(d.GetSizeKiB()), WithFormat(d.GetFormat()))
		if err != nil {
			return err
		}
	}

	if !vmConfig.StaticDHCP {
		// Add DHCP entry after defining the VM to ensure that it can be
		// removed when removing the VM, but before starting it to ensure that
		// it gets the correct IP address
		err = v.AddDHCPHost(mac, vmConfig.ID)
		if err != nil {
			return err
		}
	}

	log.Debug("Start VM")
	err = v.libvirt.DomainCreate(d)
	if err != nil {
		return fmt.Errorf("could not create (start) domain: %w", err)
	}

	return nil
}

func diskVolumeName(vmName, diskName string) string {
	return vmName + "-" + diskName
}

// WaitVmReady repeatedly tries to connect to a VM and checks if it's ready to be used.
func (v *Virter) WaitVmReady(ctx context.Context, shellClientBuilder ShellClientBuilder, vmName string, readyConfig VmReadyConfig) error {
	logger := log.WithField("vm", vmName)
	access, err := v.getVMAccess(vmName)
	if err != nil {
		return err
	}

	hostPort := net.JoinHostPort(access.ip, "ssh")

	meta, err := v.getMetaForVM(vmName)
	if err != nil {
		return fmt.Errorf("failed to fetch host keys: %w", err)
	}

	var hostKey ssh.PublicKey
	hostkeyCheck := func(_ string, _ net.Addr, key ssh.PublicKey) error {
		hostKey = key
		return nil
	}
	_, supportedAlgos := sshkeys.NewKnownHosts().AsHostKeyConfig()
	recordHostKey := meta.SSHTrustOnFirstUse && meta.HostKey == ""
	if !recordHostKey {
		knownHosts, err := v.getKnownHostsFor(access)
		if err != nil {
			return fmt.Errorf("failed to fetch host keys: %w", err)
		}

		hostkeyCheck, supportedAlgos = knownHosts.AsHostKeyConfig()
	}

	remoteUser := v.getSSHUserName(vmName)

	sshConfig := ssh.ClientConfig{
		Auth:              v.sshkeys.Auth(),
		Timeout:           readyConfig.CheckTimeout,
		User:              remoteUser,
		HostKeyCallback:   hostkeyCheck,
		HostKeyAlgorithms: supportedAlgos,
	}

	waitCtx, attempts := ctx, readyConfig.Retries
	if readyConfig.Timeout > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, readyConfig.Timeout)
		defer cancel()
		attempts = math.MaxInt
	}

	var lastErr error
	readyFunc := func() error {
		sshClient := shellClientBuilder.NewShellClient(hostPort, sshConfig)
		if err := sshClient.DialContext(waitCtx); err != nil {
			logger.Debugf("SSH dial attempt failed: %v", err)
			return err
		}
		defer sshClient.Close()
		if !meta.SkipCloudInitCheck {
			if err := sshClient.ExecScript("test -f /run/cloud-init/result.json"); err != nil {
				logger.Debugf("cloud-init not done: %v", err)
				return err
			}
		}

		if recordHostKey {
			recorded := *meta
			recorded.HostKey = string(ssh.MarshalAuthorizedKey(hostKey))
			if err := v.setMetaForVM(vmName, &recorded); err != nil {
				return err
			}
			recordHostKey = false
			logger.Debugf("Recorded host key of VM '%s'", vmName)
		}

		return nil
	}

	log.Debugf("Waiting for VM '%s' to get ready", vmName)
	start := time.Now()

	progressCtx, stopProgress := context.WithCancel(ctx)
	var progress sync.WaitGroup
	progress.Go(func() {
		logReadyProgress(progressCtx, vmName, start, readyProgressInterval)
	})

	// Using ActualTime breaks the expectation of the unit tests
	// that this code does not sleep, but we work around that by
	// always making the first ping successful in tests
	deadline, hasDeadline := waitCtx.Deadline()
	err = actualtime.ActualTime{}.Ping(waitCtx, attempts, readyConfig.CheckTimeout, func() error {
		err := readyFunc()
		// A dial cut off by the deadline reports "i/o timeout", which hides the real failure
		if err != nil && (!hasDeadline || time.Now().Before(deadline)) {
			lastErr = err
		}
		return err
	})
	stopProgress()
	progress.Wait()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("VM not ready: %w", ctx.Err())
		}
		if lastErr != nil {
			err = lastErr
		}
		return fmt.Errorf("VM '%s' not ready after %s (time.ready_timeout): %w", vmName, time.Since(start).Round(time.Second), err)
	}

	elapsed := time.Since(start)
	logReady := log.Debugf
	if elapsed >= readyProgressInterval {
		logReady = log.Infof
	}
	logReady("VM '%s' ready after %s", vmName, elapsed.Round(time.Second))
	return nil
}

const readyProgressInterval = 30 * time.Second

func logReadyProgress(ctx context.Context, vmName string, start time.Time, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case t := <-ticker.C:
			log.Infof("Still waiting for VM '%s' to get ready (%s elapsed)", vmName, t.Sub(start).Round(time.Second))
		case <-ctx.Done():
			return
		}
	}
}

// VMRm removes a VM.
func (v *Virter) VMRm(vmName string, removeDHCPEntries bool, removeBoot bool) error {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if err != nil {
		if hasErrorCode(err, libvirt.ErrNoDomain) {
			return nil
		}

		return fmt.Errorf("could not get domain: %w", err)
	}

	active, err := v.libvirt.DomainIsActive(domain)
	if err != nil {
		return fmt.Errorf("could not check if domain is active: %w", err)
	}

	persistent, err := v.libvirt.DomainIsPersistent(domain)
	if err != nil {
		return fmt.Errorf("could not check if domain is persistent: %w", err)
	}

	// Stop the VM before removing the resources it depends on. But only if
	// it is active (running). And only if it is persistent, otherwise the
	// domain is gone and we cannot query what resources it depended on.
	if active > 0 && persistent > 0 {
		log.Debug("Stop VM")
		err = v.libvirt.DomainDestroy(domain)
		if err != nil {
			return fmt.Errorf("could not destroy domain: %w", err)
		}
	}

	err = v.removeDomainDHCP(domain, removeDHCPEntries)
	if err != nil {
		return err
	}

	err = v.rmSnapshots(domain)
	if err != nil {
		return err
	}

	disks, err := v.getDisksOfDomain(domain)
	if err != nil {
		return err
	}

	for _, disk := range disks {
		if disk.shareable {
			// shared disks may be attached to other VMs and have
			// their own lifecycle, managed by "virter disk"
			continue
		}

		if !removeBoot && disk.volumeName == DynamicLayerName(vmName) {
			// do not delete boot volume
			continue
		}

		err = v.rmVolume(disk)
		if err != nil {
			return err
		}
	}

	if persistent > 0 {
		log.Debug("Undefine VM")
		err = v.libvirt.DomainUndefineFlags(domain, libvirt.DomainUndefineNvram)
		if err != nil {
			return fmt.Errorf("could not undefine domain: %w", err)
		}
	} else if active > 0 {
		// Stop the VM if we did not stop it previously.
		log.Debug("Stop VM")
		err = v.libvirt.DomainDestroy(domain)
		if err != nil {
			return fmt.Errorf("could not destroy domain: %w", err)
		}
	}

	return nil
}

func (v *Virter) rmVolume(disk VMDisk) error {
	var pool libvirt.StoragePool
	var err error
	if disk.poolName == v.provisionStoragePool.Name {
		pool = v.provisionStoragePool
	} else {
		pool, err = v.libvirt.StoragePoolLookupByName(disk.poolName)
		if err != nil {
			return fmt.Errorf("failed to lookup libvirt pool %s: %w", disk.poolName, err)
		}
	}
	layer, err := v.FindRawLayer(disk.volumeName, pool)
	if err != nil {
		if hasErrorCode(err, libvirt.ErrNoStorageVol) {
			return nil
		}

		return fmt.Errorf("could not get volume %s: %w", disk.volumeName, err)
	}

	if _, err := layer.DeleteAllIfUnused(); err != nil {
		return fmt.Errorf("could not delete volume %s: %w", disk.volumeName, err)
	}

	return nil
}

func (v *Virter) rmSnapshots(domain libvirt.Domain) error {
	snapshots, _, err := v.libvirt.DomainListAllSnapshots(domain, -1, 0)
	if err != nil {
		return fmt.Errorf("could not list snapshots: %w", err)
	}

	for _, snapshot := range snapshots {
		log.WithField("name", snapshot.Name).Debug("Delete snapshot")
		err = v.libvirt.DomainSnapshotDelete(snapshot, 0)
		if err != nil {
			return fmt.Errorf("could not delete snapshot: %w", err)
		}
	}

	return nil
}

// CommitConfig contains the configuration for committing a VM
type CommitConfig struct {
	ImageName       string
	Shutdown        bool
	ShutdownTimeout time.Duration
	ResetMachineID  bool
}

// VMCommit commits a VM to an image. If shutdown is true, the VM is shut down
// before committing. If shutdown is false, the caller is responsible for
// ensuring that the VM is not running.
func (v *Virter) VMCommit(ctx context.Context, afterNotifier AfterNotifier, vmName string, commitConfig CommitConfig, staticDHCP bool, opts ...LayerOperationOption) error {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if err != nil {
		return fmt.Errorf("could not get domain: %w", err)
	}

	active, err := v.libvirt.DomainIsActive(domain)
	if err != nil {
		return fmt.Errorf("could not check if domain is active: %w", err)
	}

	running := active != 0

	// Check if shutdown is allowed before resetting machine ID.
	if running && !commitConfig.Shutdown {
		return fmt.Errorf("must allow shutdown to commit a running VM")
	}

	if commitConfig.ResetMachineID {
		if !running {
			return fmt.Errorf("cannot reset machine ID of VM '%s' that is not running", vmName)
		}

		// Starting the VM creates a machine ID.
		// We want these IDs to be unique, so reset to empty.
		err = v.VMExecShell(
			ctx, []string{vmName},
			&ProvisionShellStep{Script: "CMD='truncate -c -s 0 /etc/machine-id'; [ \"$(id -u)\" -eq 0 ] && $CMD || { ESC=$(command -v sudo || command -v doas || command -v please || echo ''); $ESC $CMD; }"})
		if err != nil {
			return err
		}
	}

	if running && commitConfig.Shutdown {
		err = v.vmShutdown(ctx, afterNotifier, commitConfig.ShutdownTimeout, domain)
		if err != nil {
			return err
		}
	}

	err = v.VMRm(vmName, !staticDHCP, false)
	if err != nil {
		return err
	}

	layer, err := v.FindDynamicLayer(vmName, v.provisionStoragePool)
	if err != nil {
		return err
	}

	if layer == nil {
		return fmt.Errorf("could not commit: missing root layer")
	}

	volumeLayer, err := layer.ToVolumeLayer(nil, opts...)
	if err != nil {
		return err
	}

	_, err = v.MakeImage(commitConfig.ImageName, volumeLayer, opts...)
	if err != nil {
		return err
	}

	return nil
}

// vmShutdown sends a command to libvirt to shut down a domain. It then waits for
// the domain to no longer be active, or until shutdownTimeout has elapsed.
// Note that we don't use libvirt's LifetimeEvents here, because its event
// receiving loop sometimes causes a deadlock for us.
func (v *Virter) vmShutdown(ctx context.Context, afterNotifier AfterNotifier, shutdownTimeout time.Duration, domain libvirt.Domain) error {
	active, err := v.libvirt.DomainIsActive(domain)
	if err != nil {
		return fmt.Errorf("could not check if domain is active: %w", err)
	}

	if active != 0 {
		log.Debug("Shut down VM")

		err = v.libvirt.DomainShutdown(domain)
		if err != nil {
			return fmt.Errorf("could not shut down domain: %w", err)
		}

		log.Debug("Wait for VM to stop")
	}

	timeout := afterNotifier.After(shutdownTimeout)

	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for active != 0 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("error while waiting for domain to stop: %v", ctx.Err())
		case <-timeout:
			return fmt.Errorf("timed out waiting for domain to stop")
		case <-tick.C:
			log.Debugf("Polling for VM shutdown")
			active, err = v.libvirt.DomainIsActive(domain)
			if err != nil {
				log.Warnf("Error while polling for VM shutdown: could not check if domain %q is active: %v", domain.Name, err)
			}
		}
	}

	return nil
}

// vmAccess describes how Virter reaches a running VM: the description of the
// network that the VM's access interface is on (see getAccessNIC), and the
// IP address of that interface.
type vmAccess struct {
	name    string
	network *lx.Network
	ip      string
}

func (v *Virter) getVMAccess(vmName string) (vmAccess, error) {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if err != nil {
		return vmAccess{}, fmt.Errorf("could not get domain '%s': %w", vmName, err)
	}

	active, err := v.libvirt.DomainIsActive(domain)
	if err != nil {
		return vmAccess{}, fmt.Errorf("could not check if domain '%s' is active: %w", vmName, err)
	}

	if active == 0 {
		return vmAccess{}, fmt.Errorf("cannot exec against VM '%s' that is not running", vmName)
	}

	domainDescription, err := getDomainDescription(v.libvirt, domain)
	if err != nil {
		return vmAccess{}, fmt.Errorf("could not get description of domain '%s': %w", vmName, err)
	}

	// Determine the VM's access network from its first interface, rather than
	// assuming it is on the configured provision network. This lets commands
	// like 'vm ssh' reach VMs that were created on a different network.
	accessNIC, err := getAccessNIC(vmName, domainDescription)
	if err != nil {
		return vmAccess{}, err
	}

	network, err := v.libvirt.NetworkLookupByName(accessNIC.Network)
	if err != nil {
		return vmAccess{}, &LibvirtGetError{
			Message:  fmt.Sprintf("could not look up access network '%s' for VM '%s'", accessNIC.Network, vmName),
			Err:      err,
			NotFound: hasErrorCode(err, libvirt.ErrNoNetwork),
		}
	}

	networkDescription, err := getNetworkDescription(v.libvirt, network)
	if err != nil {
		return vmAccess{}, fmt.Errorf("could not get access network '%s' for VM '%s': %w", accessNIC.Network, vmName, err)
	}

	ips := findIPs(networkDescription, accessNIC.MAC)
	if len(ips) < 1 {
		return vmAccess{}, fmt.Errorf("could not find IP for VM '%s' in network '%s'", vmName, accessNIC.Network)
	}

	return vmAccess{name: vmName, network: networkDescription, ip: ips[0]}, nil
}

func (v *Virter) getVMAccesses(vmNames []string) ([]vmAccess, error) {
	var accesses []vmAccess

	for _, vmName := range vmNames {
		access, err := v.getVMAccess(vmName)
		if err != nil {
			return nil, err
		}
		accesses = append(accesses, access)
	}
	return accesses, nil
}

// getAccessNIC returns the VM's first network interface, which Virter treats as
// the access network interface. It is an error if the VM has no interfaces, if
// the first interface is not on a libvirt network, or if it has no MAC address.
func getAccessNIC(vmName string, domainDescription *lx.Domain) (nic, error) {
	nics, err := getDomainNICs(domainDescription)
	if err != nil {
		return nic{}, fmt.Errorf("could not get interfaces for VM '%s': %w", vmName, err)
	}

	if len(nics) < 1 {
		return nic{}, fmt.Errorf("no interfaces found for VM '%s'", vmName)
	}

	if nics[0].Network == "" {
		return nic{}, fmt.Errorf("no network found for VM '%s'", vmName)
	}

	if nics[0].MAC == "" {
		return nic{}, fmt.Errorf("no MAC address found for VM '%s'", vmName)
	}

	return nics[0], nil
}

func (v *Virter) getKnownHostsFor(accesses ...vmAccess) (sshkeys.KnownHosts, error) {
	knownHosts := sshkeys.NewKnownHosts()
	for _, access := range accesses {
		vmName := access.name
		meta, err := v.getMetaForVM(vmName)
		if err != nil {
			return nil, err
		}

		if meta.HostKey == "" {
			return nil, fmt.Errorf("host key of VM '%s' not recorded yet, wait for it with 'virter vm wait-ready %s'", vmName, vmName)
		}

		hosts := []string{access.ip, vmName}

		domainSuffix := getDomainSuffix(access.network)
		if domainSuffix != "" {
			hosts = append(hosts, fmt.Sprintf("%s.%s", vmName, domainSuffix))
		}
		knownHosts.AddHost(meta.HostKey, hosts...)
	}

	return knownHosts, nil
}

func (v *Virter) VMGetKnownHosts(vmName string) (string, error) {
	access, err := v.getVMAccess(vmName)
	if err != nil {
		return "", fmt.Errorf("failed to fetch host keys: %w", err)
	}

	knownHosts, err := v.getKnownHostsFor(access)
	if err != nil {
		return "", fmt.Errorf("failed to fetch host keys: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := knownHosts.AsKnownHostsFile(buf); err != nil {
		return "", fmt.Errorf("failed to write known hosts file: %w", err)
	}
	return buf.String(), nil
}

func (v *Virter) getSSHUserName(vmName string) string {
	meta, err := v.getMetaForVM(vmName)

	/* VM created with an older virter? */
	if err != nil || meta.SSHUserName == "" {
		return "root"
	}

	return meta.SSHUserName
}

func (v *Virter) getSSHUserNames(vmNames []string) []string {
	var vmSSHUserNames []string

	for _, vmName := range vmNames {
		SSHUserName := v.getSSHUserName(vmName)
		vmSSHUserNames = append(vmSSHUserNames, SSHUserName)
	}
	return vmSSHUserNames
}

// VMExecContainer runs a container against some VMs.
func (v *Virter) VMExecContainer(ctx context.Context, containerProvider containerapi.ContainerProvider,
	vmNames []string, containerCfg *containerapi.ContainerConfig, copyStep *ProvisionContainerCopyStep) error {

	networkDescription, err := getNetworkDescription(v.libvirt, v.provisionNetwork)
	if err != nil {
		return fmt.Errorf("could not get network: %w", err)
	}

	accessIPNet, err := getIPNet(networkDescription)
	if err != nil {
		return fmt.Errorf("could not get network: %w", err)
	}

	containerCfg.SetEnv("VIRTER_ACCESS_NETWORK", accessIPNet.String())

	accesses, err := v.getVMAccesses(vmNames)
	if err != nil {
		return err
	}

	vmSSHUserNames := v.getSSHUserNames(vmNames)

	domain := getDomainSuffix(networkDescription)

	var ips []string
	for _, access := range accesses {
		ips = append(ips, access.ip)
		containerCfg.AddExtraHost(containerapi.ExtraHost{HostName: access.name, IP: access.ip})
		if domain != "" {
			containerCfg.AddExtraHost(containerapi.ExtraHost{HostName: fmt.Sprintf("%s.%s", access.name, domain), IP: access.ip})
		}
	}

	knownHosts, err := v.getKnownHostsFor(accesses...)
	if err != nil {
		return err
	}

	if domain != "" {
		containerCfg.AddDNSSearchDomain(domain)
	}
	// The libvirt DNS server listens on the host address of the network
	containerCfg.AddDNSServer(accessIPNet.IP)

	err = containerRun(ctx, containerProvider, containerCfg, vmNames, vmSSHUserNames, ips, v.sshkeys, knownHosts, copyStep)
	if err != nil {
		return fmt.Errorf("failed to run container provisioning: %w", err)
	}

	return nil
}

// VMSSHSession runs an interactive shell session in a VM
func (v *Virter) VMSSHSession(ctx context.Context, vmName string) error {
	access, err := v.getVMAccess(vmName)
	if err != nil {
		return err
	}

	knownHosts, err := v.getKnownHostsFor(access)
	if err != nil {
		return fmt.Errorf("failed to fetch host keys: %w", err)
	}

	hostkeyCheck, supportedAlgos := knownHosts.AsHostKeyConfig()

	remoteUser := v.getSSHUserName(vmName)

	sshConfig := ssh.ClientConfig{
		Auth:              v.sshkeys.Auth(),
		User:              remoteUser,
		HostKeyCallback:   hostkeyCheck,
		HostKeyAlgorithms: supportedAlgos,
	}

	hostPort := net.JoinHostPort(access.ip, "22")
	sshClient := sshclient.NewSSHClient(hostPort, sshConfig)
	if err := sshClient.DialContext(ctx); err != nil {
		return err
	}
	defer sshClient.Close()

	return sshClient.Shell()
}

// VMExecShell runs a simple shell command against some VMs.
func (v *Virter) VMExecShell(ctx context.Context, vmNames []string, shellStep *ProvisionShellStep) error {
	accesses, err := v.getVMAccesses(vmNames)
	if err != nil {
		return err
	}

	knownHosts, err := v.getKnownHostsFor(accesses...)
	if err != nil {
		return err
	}

	hostkeyCheck, supportedAlgos := knownHosts.AsHostKeyConfig()

	var g errgroup.Group
	for _, access := range accesses {
		ip := access.ip
		vmName := access.name

		remoteUser := v.getSSHUserName(vmName)
		sshConfig := ssh.ClientConfig{
			Auth:              v.sshkeys.Auth(),
			User:              remoteUser,
			HostKeyCallback:   hostkeyCheck,
			HostKeyAlgorithms: supportedAlgos,
		}

		log.Debugln("Provisioning via SSH:", shellStep.Script, "in", ip)
		g.Go(func() error {
			return runSSHCommand(ctx, &sshConfig, vmName, net.JoinHostPort(ip, "22"), shellStep.Script, EnvmapToSlice(shellStep.Env))
		})
	}

	return g.Wait()
}

func (v *Virter) VMExecRsync(ctx context.Context, copier netcopy.NetworkCopier, vmNames []string, rsyncStep *ProvisionRsyncStep) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Check the non-glob prefix of the pattern first, so that patterns
	// pointing outside the working directory are rejected even when no
	// files match. This mirrors Docker/BuildKit's approach for COPY.
	globPrefix, _ := splitGlobPrefix(rsyncStep.Source)
	if err := checkPathInWorkDir(globPrefix, wd); err != nil {
		return fmt.Errorf("rsync source not allowed: %w", err)
	}

	files, err := filepath.Glob(rsyncStep.Source)
	if err != nil {
		return fmt.Errorf("failed to parse glob pattern: %w", err)
	}

	resolved := make([]string, 0, len(files))
	for _, f := range files {
		realPath, err := filepath.EvalSymlinks(f)
		if err != nil {
			return fmt.Errorf("failed to resolve path %q: %w", f, err)
		}
		if err := checkPathInWorkDir(realPath, wd); err != nil {
			return fmt.Errorf("rsync source not allowed: %w", err)
		}
		// Preserve trailing slash: it is semantically significant for
		// rsync (copy directory contents vs. copy directory itself).
		if strings.HasSuffix(f, string(filepath.Separator)) && !strings.HasSuffix(realPath, string(filepath.Separator)) {
			realPath += string(filepath.Separator)
		}
		resolved = append(resolved, realPath)
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, vmName := range vmNames {
		vmName := vmName
		log.Debugf(`Copying files via rsync: %s to %s on %s`, rsyncStep.Source, rsyncStep.Dest, vmName)
		g.Go(func() error {
			dest := fmt.Sprintf("%s:%s", vmName, rsyncStep.Dest)
			return v.VMExecCopy(ctx, copier, resolved, dest)
		})
	}
	return g.Wait()
}

func (v *Virter) VMExecCopy(ctx context.Context, copier netcopy.NetworkCopier, sourceSpecs []string, destSpec string) error {
	sources := make([]netcopy.HostPath, len(sourceSpecs))
	var accesses []vmAccess
	for i, srcSpec := range sourceSpecs {
		sources[i] = netcopy.ParseHostPath(srcSpec)

		if !sources[i].Local() {
			sources[i].User = v.getSSHUserName(sources[i].Host)

			access, err := v.getVMAccess(sources[i].Host)
			if err != nil {
				return err
			}
			accesses = append(accesses, access)
			// Replace hostname with ip
			sources[i].Host = access.ip
		}
	}

	dest := netcopy.ParseHostPath(destSpec)
	if !dest.Local() {
		dest.User = v.getSSHUserName(dest.Host)

		access, err := v.getVMAccess(dest.Host)
		if err != nil {
			return err
		}
		accesses = append(accesses, access)
		dest.Host = access.ip
	}

	knownHosts, err := v.getKnownHostsFor(accesses...)
	if err != nil {
		return err
	}

	return copier.Copy(ctx, sources, dest, v.sshkeys, knownHosts)
}

func runSSHCommand(ctx context.Context, config *ssh.ClientConfig, vmName, ipPort, script string, env []string) error {
	script, err := sshclient.AddEnv(script, env)
	if err != nil {
		return err
	}

	// Retry connection until the context is cancelled. We expect to have
	// already formed a successful SSH connection before we do any
	// provisioning over SSH. This is a workaround for VMs that make SSH
	// available but then temporarily stop it again.
	sshClient, err := connectSSHRetry(ctx, config, ipPort)
	if err != nil {
		return err
	}
	defer sshClient.Close()

	outp, err := sshClient.StdoutPipe()
	if err != nil {
		return err
	}
	errp, err := sshClient.StderrPipe()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go logLines(&wg, vmName, false, outp)
	go logLines(&wg, vmName, true, errp)

	err = sshClient.ExecScript(script)
	wg.Wait()

	return err
}

func connectSSHRetry(ctx context.Context, config *ssh.ClientConfig, ipPort string) (*sshclient.SSHClient, error) {
	var sshClient *sshclient.SSHClient
	for sshClient == nil {
		sshClient = sshclient.NewSSHClient(ipPort, *config)
		if err := sshClient.DialContext(ctx); err != nil {
			select {
			case <-ctx.Done():
				return nil, err
			case <-time.After(time.Second):
			}
			log.Warnf("Retrying SSH connection due to failure: %v", err)
			sshClient = nil
		}
	}
	return sshClient, nil
}
