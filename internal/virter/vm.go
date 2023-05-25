package virter

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/LINBIT/containerapi"
	sshclient "github.com/LINBIT/gosshclient"
	libvirt "github.com/digitalocean/go-libvirt"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"

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
		imgs = append(imgs, imageAndPool{diskVolumeName(vmName, d.GetName()), v.provisionStoragePool})
	}

	for _, img := range imgs {
		if layer, err := v.FindDynamicLayer(img.image, img.pool); layer != nil || err != nil {
			return layer != nil, err
		}
	}
	return false, nil
}

func (v *Virter) ListVM() ([]string, error) {
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

	_, err = v.libvirt.ConnectGetDomainCapabilities(nil, []string{vmConfig.CpuArch.QemuArch()}, machine, nil, 0)
	if err != nil {
		return fmt.Errorf("host does not support emulating %s. install qemu-system-%s", vmConfig.CpuArch, vmConfig.CpuArch.QemuArch())
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

	log.Print("Create host key")
	hostkey, err := sshkeys.NewRSAHostKey()
	if err != nil {
		return fmt.Errorf("could not create new host key: %w", err)
	}

	meta := &VMMeta{
		HostKey:     hostkey.PublicKey(),
		SSHUserName: vmConfig.SSHUserName,
	}

	vmXML, err := v.vmXML(vmConfig, mac, meta)
	if err != nil {
		return err
	}

	log.Debugf("Using domain XML: %s", vmXML)

	log.Print("Define VM")
	d, err := v.libvirt.DomainDefineXML(vmXML)
	if err != nil {
		return fmt.Errorf("could not define domain: %w", err)
	}

	log.Print("Create boot volume")
	_, err = v.ImageSpawn(vmConfig.Name, v.provisionStoragePool, vmConfig.Image, vmConfig.BootCapacityKiB)
	if err != nil {
		return err
	}

	log.Print("Create cloud-init volume")
	_, err = v.createCIData(vmConfig, hostkey)
	if err != nil {
		return err
	}

	for _, d := range vmConfig.Disks {
		log.Printf("Create volume '%s'", d.GetName())
		_, err = v.NewDynamicLayer(diskVolumeName(vmConfig.Name, d.GetName()), v.provisionStoragePool, WithCapacity(d.GetSizeKiB()), WithFormat(d.GetFormat()))
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

	log.Print("Start VM")
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
	ips, err := v.getIPs([]string{vmName})
	if err != nil {
		return err
	}
	if len(ips) != 1 {
		return fmt.Errorf("Expected a single IP")
	}

	hostPort := net.JoinHostPort(ips[0], "ssh")

	knownHosts, err := v.getKnownHostsFor(vmName)
	if err != nil {
		return fmt.Errorf("failed to fetch host keys: %w", err)
	}

	hostkeyCheck, supportedAlgos := knownHosts.AsHostKeyConfig()

	remoteUser := v.getSSHUserName(vmName)

	sshConfig := ssh.ClientConfig{
		Auth:              v.sshkeys.Auth(),
		Timeout:           readyConfig.CheckTimeout,
		User:              remoteUser,
		HostKeyCallback:   hostkeyCheck,
		HostKeyAlgorithms: supportedAlgos,
	}

	readyFunc := func() error {
		sshClient := shellClientBuilder.NewShellClient(hostPort, sshConfig)
		if err := sshClient.DialContext(ctx); err != nil {
			log.Debugf("SSH dial attempt failed: %v", err)
			return err
		}
		defer sshClient.Close()
		if err := sshClient.ExecScript("test -f /run/cloud-init/result.json"); err != nil {
			log.Debugf("cloud-init not done: %v", err)
			return err
		}

		return nil
	}

	log.Print("Wait for VM to get ready")

	// Using ActualTime breaks the expectation of the unit tests
	// that this code does not sleep, but we work around that by
	// always making the first ping successful in tests
	if err := (actualtime.ActualTime{}.Ping(ctx, readyConfig.Retries, readyConfig.CheckTimeout, readyFunc)); err != nil {
		return fmt.Errorf("VM not ready: %w", err)
	}

	log.Print("Successfully connected to ready VM")
	return nil
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
		log.Print("Stop VM")
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
		log.Print("Undefine VM")
		err = v.libvirt.DomainUndefineFlags(domain, libvirt.DomainUndefineNvram)
		if err != nil {
			return fmt.Errorf("could not undefine domain: %w", err)
		}
	} else if active > 0 {
		// Stop the VM if we did not stop it previously.
		log.Print("Stop VM")
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

	err = layer.DeleteAllIfUnused()
	if err != nil {
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
		log.Printf("Delete snapshot %v", snapshot.Name)
		err = v.libvirt.DomainSnapshotDelete(snapshot, 0)
		if err != nil {
			return fmt.Errorf("could not delete snapshot: %w", err)
		}
	}

	return nil
}

// VMCommit commits a VM to an image. If shutdown is true, the VM is shut down
// before committing. If shutdown is false, the caller is responsible for
// ensuring that the VM is not running.
func (v *Virter) VMCommit(ctx context.Context, afterNotifier AfterNotifier, vmName, imageName string, shutdown bool, shutdownTimeout time.Duration, staticDHCP bool, opts ...LayerOperationOption) error {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if err != nil {
		return fmt.Errorf("could not get domain: %w", err)
	}

	if shutdown {
		err = v.vmShutdown(ctx, afterNotifier, shutdownTimeout, domain)
		if err != nil {
			return err
		}
	} else {
		active, err := v.libvirt.DomainIsActive(domain)
		if err != nil {
			return fmt.Errorf("could not check if domain is active: %w", err)
		}

		if active != 0 {
			return fmt.Errorf("cannot commit a running VM")
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

	_, err = v.MakeImage(imageName, volumeLayer, opts...)
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
		log.Printf("Shut down VM")

		err = v.libvirt.DomainShutdown(domain)
		if err != nil {
			return fmt.Errorf("could not shut down domain: %w", err)
		}

		log.Printf("Wait for VM to stop")
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

func (v *Virter) getIP(vmName string, network *libvirt.Network) (string, error) {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if err != nil {
		return "", fmt.Errorf("could not get domain '%s': %w", vmName, err)
	}

	active, err := v.libvirt.DomainIsActive(domain)
	if err != nil {
		return "", fmt.Errorf("could not check if domain '%s' is active: %w", vmName, err)
	}

	if active == 0 {
		return "", fmt.Errorf("cannot exec against VM '%s' that is not running", vmName)
	}

	if network == nil {
		network = &v.provisionNetwork
	}

	ip, err := v.findVMIP(*network, domain)
	if err != nil {
		return "", fmt.Errorf("could not find IP for VM '%s': %w", vmName, err)
	}

	return ip, nil
}

func (v *Virter) getIPs(vmNames []string) ([]string, error) {
	var ips []string

	for _, vmName := range vmNames {
		ip, err := v.getIP(vmName, &v.provisionNetwork)
		if err != nil {
			return nil, err
		}
		ips = append(ips, ip)
	}
	return ips, nil
}

func (v *Virter) getKnownHostsFor(vmNames ...string) (sshkeys.KnownHosts, error) {
	ips, err := v.getIPs(vmNames)
	if err != nil {
		return nil, err
	}

	domainSuffix, err := v.getDomainSuffix()
	if err != nil {
		return nil, err
	}

	knownHosts := sshkeys.NewKnownHosts()
	for i, vmName := range vmNames {
		meta, err := v.getMetaForVM(vmName)
		if err != nil {
			return nil, err
		}

		hosts := []string{ips[i], vmName}
		if domainSuffix != "" {
			hosts = append(hosts, fmt.Sprintf("%s.%s", vmName, domainSuffix))
		}
		knownHosts.AddHost(meta.HostKey, hosts...)
	}

	return knownHosts, nil
}

func (v *Virter) VMGetKnownHosts(vmName string) (string, error) {
	knownHosts, err := v.getKnownHostsFor(vmName)
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
	ips, err := v.getIPs(vmNames)
	if err != nil {
		return err
	}

	vmSSHUserNames := v.getSSHUserNames(vmNames)

	domain, err := v.getDomainSuffix()
	if err != nil {
		return err
	}

	for i := range ips {
		containerCfg.AddExtraHost(containerapi.ExtraHost{HostName: vmNames[i], IP: ips[i]})
		if domain != "" {
			containerCfg.AddExtraHost(containerapi.ExtraHost{HostName: fmt.Sprintf("%s.%s", vmNames[i], domain), IP: ips[i]})
		}
	}

	knownHosts, err := v.getKnownHostsFor(vmNames...)
	if err != nil {
		return err
	}

	dnsserver, err := v.getDNSServer()
	if err != nil {
		return err
	}

	if domain != "" {
		containerCfg.AddDNSSearchDomain(domain)
	}
	containerCfg.AddDNSServer(dnsserver)

	err = containerRun(ctx, containerProvider, containerCfg, vmNames, vmSSHUserNames, v.sshkeys, knownHosts, copyStep)
	if err != nil {
		return fmt.Errorf("failed to run container provisioning: %w", err)
	}

	return nil
}

// VMSSHSession runs an interactive shell session in a VM
func (v *Virter) VMSSHSession(ctx context.Context, vmName string) error {
	ips, err := v.getIPs([]string{vmName})
	if err != nil {
		return err
	}
	if len(ips) != 1 {
		return fmt.Errorf("Expected a single IP")
	}

	knownHosts, err := v.getKnownHostsFor(vmName)
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

	hostPort := net.JoinHostPort(ips[0], "22")
	sshClient := sshclient.NewSSHClient(hostPort, sshConfig)
	if err := sshClient.DialContext(ctx); err != nil {
		return err
	}
	defer sshClient.Close()

	return sshClient.Shell()
}

// VMExecShell runs a simple shell command against some VMs.
func (v *Virter) VMExecShell(ctx context.Context, vmNames []string, shellStep *ProvisionShellStep) error {
	ips, err := v.getIPs(vmNames)
	if err != nil {
		return err
	}

	knownHosts, err := v.getKnownHostsFor(vmNames...)
	if err != nil {
		return err
	}

	hostkeyCheck, supportedAlgos := knownHosts.AsHostKeyConfig()

	var g errgroup.Group
	for i, ip := range ips {
		ip := ip
		vmName := vmNames[i]

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
	files, err := filepath.Glob(rsyncStep.Source)
	if err != nil {
		return fmt.Errorf("failed to parse glob pattern: %w", err)
	}

	g, ctx := errgroup.WithContext(ctx)
	for _, vmName := range vmNames {
		vmName := vmName
		log.Printf(`Copying files via rsync: %s to %s on %s`, rsyncStep.Source, rsyncStep.Dest, vmName)
		g.Go(func() error {
			dest := fmt.Sprintf("%s:%s", vmName, rsyncStep.Dest)
			return v.VMExecCopy(ctx, copier, files, dest)
		})
	}
	return g.Wait()
}

func (v *Virter) VMExecCopy(ctx context.Context, copier netcopy.NetworkCopier, sourceSpecs []string, destSpec string) error {
	sources := make([]netcopy.HostPath, len(sourceSpecs))
	var vmNames []string
	for i, srcSpec := range sourceSpecs {
		sources[i] = netcopy.ParseHostPath(srcSpec)

		if !sources[i].Local() {
			sources[i].User = v.getSSHUserName(sources[i].Host)

			vmNames = append(vmNames, sources[i].Host)
			// Replace hostname with ip
			ip, err := v.getIP(sources[i].Host, nil)
			if err != nil {
				return err
			}
			sources[i].Host = ip
		}
	}

	dest := netcopy.ParseHostPath(destSpec)
	if !dest.Local() {
		dest.User = v.getSSHUserName(dest.Host)

		vmNames = append(vmNames, dest.Host)
		ip, err := v.getIP(dest.Host, nil)
		if err != nil {
			return err
		}
		dest.Host = ip
	}

	knownHosts, err := v.getKnownHostsFor(vmNames...)
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

func (v *Virter) findVMIP(network libvirt.Network, domain libvirt.Domain) (string, error) {
	nics, err := v.getNICs(domain)
	if err != nil {
		return "", err
	}

	mac := ""
	for _, nic := range nics {
		if network.Name == nic.Network {
			mac = nic.MAC
			break
		}
	}
	if mac == "" {
		return "", fmt.Errorf("could not find MAC address of domain")
	}

	ips, err := v.findIPs(network, mac)
	if err != nil {
		return "", err
	}
	if len(ips) < 1 {
		return "", fmt.Errorf("no IP found for domain")
	}

	return ips[0], nil
}
