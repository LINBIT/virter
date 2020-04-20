package virter

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	libvirt "github.com/digitalocean/go-libvirt"
)

// VMRun starts a VM.
func (v *Virter) VMRun(g ISOGenerator, waiter PortWaiter, vmConfig VMConfig, waitSSH bool) error {
	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	log.Print("Create boot volume")
	err = v.createVMVolume(sp, vmConfig)
	if err != nil {
		return err
	}

	log.Print("Create cloud-init volume")
	err = v.createCIData(sp, g, vmConfig)
	if err != nil {
		return err
	}

	log.Print("Create scratch volume")
	err = v.createScratchVolume(sp, vmConfig)
	if err != nil {
		return err
	}

	ip, err := v.createVM(sp, vmConfig)
	if err != nil {
		return err
	}

	if waitSSH {
		log.Print("Wait for SSH port to open")
		err = waiter.WaitPort(ip, "ssh")
		if err != nil {
			return fmt.Errorf("unable to connect to SSH port: %w", err)
		}
		log.Print("Successfully connected to SSH port")
	}

	return nil
}

func (v *Virter) createCIData(sp libvirt.StoragePool, g ISOGenerator, vmConfig VMConfig) error {
	vmName := vmConfig.VMName
	sshPublicKeys := vmConfig.SSHPublicKeys

	metaData, err := v.metaData(vmName)
	if err != nil {
		return err
	}

	userData, err := v.userData(vmName, sshPublicKeys)
	if err != nil {
		return err
	}

	files := map[string][]byte{
		"meta-data": []byte(metaData),
		"user-data": []byte(userData),
	}

	ciData, err := g.Generate(files)
	if err != nil {
		return fmt.Errorf("failed to generate ISO: %w", err)
	}

	xml, err := v.ciDataVolumeXML(ciDataVolumeName(vmName))
	if err != nil {
		return err
	}

	sv, err := v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create cloud-init volume: %w", err)
	}

	err = v.libvirt.StorageVolUpload(sv, bytes.NewReader(ciData), 0, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to transfer cloud-init data to libvirt: %w", err)
	}

	return nil
}

func ciDataVolumeName(vmName string) string {
	return vmName + "-cidata"
}

func (v *Virter) metaData(vmName string) (string, error) {
	templateData := map[string]interface{}{
		"VMName": vmName,
	}

	return v.renderTemplate(templateMetaData, templateData)
}

func (v *Virter) userData(vmName string, sshPublicKeys []string) (string, error) {
	templateData := map[string]interface{}{
		"VMName":        vmName,
		"SSHPublicKeys": sshPublicKeys,
	}

	return v.renderTemplate(templateUserData, templateData)
}

func (v *Virter) ciDataVolumeXML(name string) (string, error) {
	templateData := map[string]interface{}{
		"VolumeName": name,
	}

	return v.renderTemplate(templateCIData, templateData)
}

func (v *Virter) createVMVolume(sp libvirt.StoragePool, vmConfig VMConfig) error {
	imageName := vmConfig.ImageName
	vmName := vmConfig.VMName

	backingVolume, err := v.libvirt.StorageVolLookupByName(sp, imageName)
	if err != nil {
		return fmt.Errorf("could not get backing image volume: %w", err)
	}

	backingPath, err := v.libvirt.StorageVolGetPath(backingVolume)
	if err != nil {
		return fmt.Errorf("could not get backing image path: %w", err)
	}

	xml, err := v.vmVolumeXML(vmName, backingPath)
	if err != nil {
		return err
	}

	_, err = v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create VM boot volume: %w", err)
	}

	return nil
}

func (v *Virter) vmVolumeXML(name string, backingPath string) (string, error) {
	templateData := map[string]interface{}{
		"VolumeName":  name,
		"BackingPath": backingPath,
	}

	return v.renderTemplate(templateVMVolume, templateData)
}

func (v *Virter) createScratchVolume(sp libvirt.StoragePool, vmConfig VMConfig) error {
	vmName := vmConfig.VMName

	xml, err := v.scratchVolumeXML(scratchVolumeName(vmName))
	if err != nil {
		return err
	}

	_, err = v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create scratch volume: %w", err)
	}

	return nil
}

func scratchVolumeName(vmName string) string {
	return vmName + "-scratch"
}

func (v *Virter) scratchVolumeXML(name string) (string, error) {
	templateData := map[string]interface{}{
		"VolumeName": name,
	}

	return v.renderTemplate(templateScratchVolume, templateData)
}

func (v *Virter) createVM(sp libvirt.StoragePool, vmConfig VMConfig) (net.IP, error) {
	vmName := vmConfig.VMName
	vmID := vmConfig.VMID
	memKiB := vmConfig.MemoryKiB
	vcpus := vmConfig.VCPUs
	mac := qemuMAC(vmID)

	xml, err := v.vmXML(sp.Name, vmName, mac, memKiB, vcpus)
	if err != nil {
		return nil, err
	}

	log.Print("Define VM")
	d, err := v.libvirt.DomainDefineXML(xml)
	if err != nil {
		return nil, fmt.Errorf("could not define domain: %w", err)
	}

	// Add DHCP entry after defining the VM to ensure that it can be
	// removed when removing the VM, but before starting it to ensure that
	// it gets the correct IP address
	ip, err := v.addDHCPEntry(mac, vmID)
	if err != nil {
		return nil, err
	}

	log.Print("Start VM")
	err = v.libvirt.DomainCreate(d)
	if err != nil {
		return nil, fmt.Errorf("could create create (start) domain: %w", err)
	}

	return ip, nil
}

func (v *Virter) vmXML(poolName string, vmName string, mac string, memKiB uint64, vcpus uint) (string, error) {
	templateData := map[string]interface{}{
		"PoolName":  poolName,
		"VMName":    vmName,
		"MAC":       mac,
		"MemoryKiB": memKiB,
		"VCPUs":     vcpus,
	}

	return v.renderTemplate(templateVM, templateData)
}

// VMRm removes a VM.
func (v *Virter) VMRm(vmName string) error {
	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	err = v.vmRmExceptBoot(sp, vmName)
	if err != nil {
		return err
	}

	err = v.rmVolume(sp, vmName, "boot")
	if err != nil {
		return err
	}

	return nil
}

func (v *Virter) vmRmExceptBoot(sp libvirt.StoragePool, vmName string) error {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if !hasErrorCode(err, errNoDomain) {
		if err != nil {
			return fmt.Errorf("could not get domain: %w", err)
		}

		err = v.rmSnapshots(domain)
		if err != nil {
			return err
		}

		active, err := v.libvirt.DomainIsActive(domain)
		if err != nil {
			return fmt.Errorf("could not check if domain is active: %w", err)
		}

		persistent, err := v.libvirt.DomainIsPersistent(domain)
		if err != nil {
			return fmt.Errorf("could not check if domain is persistent: %w", err)
		}

		err = v.rmDHCPEntry(domain)
		if err != nil {
			return err
		}

		if active != 0 {
			log.Print("Stop VM")
			err = v.libvirt.DomainDestroy(domain)
			if err != nil {
				return fmt.Errorf("could not destroy domain: %w", err)
			}
		}

		if persistent != 0 {
			log.Print("Undefine VM")
			err = v.libvirt.DomainUndefine(domain)
			if err != nil {
				return fmt.Errorf("could not undefine domain: %w", err)
			}
		}
	}

	err = v.rmVolume(sp, scratchVolumeName(vmName), "scratch")
	if err != nil {
		return err
	}

	err = v.rmVolume(sp, ciDataVolumeName(vmName), "cloud-init")
	if err != nil {
		return err
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

func (v *Virter) rmVolume(sp libvirt.StoragePool, volumeName string, debugName string) error {
	volume, err := v.libvirt.StorageVolLookupByName(sp, volumeName)
	if !hasErrorCode(err, errNoStorageVol) {
		if err != nil {
			return fmt.Errorf("could not get %v volume: %w", debugName, err)
		}

		log.Printf("Delete %v volume", debugName)
		err = v.libvirt.StorageVolDelete(volume, 0)
		if err != nil {
			return fmt.Errorf("could not delete %v volume: %w", debugName, err)
		}
	}

	return nil
}

// VMCommit commits a VM to an image. If shutdown is true, a goroutine to watch
// for events will be started. This goroutine will only terminate when the
// libvirt connection is closed, so take care of leaking goroutines.
func (v *Virter) VMCommit(afterNotifier AfterNotifier, vmName string, shutdown bool, shutdownTimeout time.Duration) error {
	domain, err := v.libvirt.DomainLookupByName(vmName)
	if err != nil {
		return fmt.Errorf("could not get domain: %w", err)
	}

	if shutdown {
		err = v.vmShutdown(afterNotifier, shutdownTimeout, domain)
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

	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	err = v.vmRmExceptBoot(sp, vmName)
	if err != nil {
		return err
	}

	return nil
}

func (v *Virter) vmShutdown(afterNotifier AfterNotifier, shutdownTimeout time.Duration, domain libvirt.Domain) error {
	events, err := v.libvirt.LifecycleEvents()
	if err != nil {
		return fmt.Errorf("could not start waiting for events: %w", err)
	}

	// Check whether domain is active after starting event stream
	// to ensure that the shutdown event is not missed.
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

	for active != 0 {
		select {
		case event := <-events:
			if event.Dom.ID == domain.ID && event.Event == int32(libvirt.DomainEventStopped) {
				log.Printf("VM stopped")
				active = 0
			}
		case <-timeout:
			return fmt.Errorf("timed out waiting for domain to stop")
		}
	}

	return nil
}

func (v *Virter) getIPs(vmNames []string) ([]string, error) {
	var ips []string
	network, err := v.libvirt.NetworkLookupByName(v.networkName)
	if err != nil {
		return ips, fmt.Errorf("could not get network: %w", err)
	}

	for _, vmName := range vmNames {
		domain, err := v.libvirt.DomainLookupByName(vmName)
		if err != nil {
			return ips, fmt.Errorf("could not get domain '%s': %w", vmName, err)
		}

		active, err := v.libvirt.DomainIsActive(domain)
		if err != nil {
			return ips, fmt.Errorf("could not check if domain '%s' is active: %w", vmName, err)
		}

		if active == 0 {
			return ips, fmt.Errorf("cannot exec against VM '%s' that is not running", vmName)
		}

		ip, err := v.findVMIP(network, domain)
		if err != nil {
			return ips, fmt.Errorf("could not find IP for VM '%s': %w", vmName, err)
		}

		ips = append(ips, ip)
	}
	return ips, nil
}

// VMExecDocker runs a docker container against some VMs.
func (v *Virter) VMExecDocker(ctx context.Context, docker DockerClient, vmNames []string, dockerContainerConfig DockerContainerConfig, sshPrivateKey []byte) error {
	ips, err := v.getIPs(vmNames)
	if err != nil {
		return err
	}

	return dockerRun(ctx, docker, dockerContainerConfig, ips, sshPrivateKey)
}

// VMExecShell runs a simple shell command against some VMs.
func (v *Virter) VMExecShell(ctx context.Context, vmNames []string, sshPrivateKey []byte, shellStep *ProvisionShellStep) error {
	ips, err := v.getIPs(vmNames)
	if err != nil {
		return err
	}

	signer, err := ssh.ParsePrivateKey(sshPrivateKey)
	if err != nil {
		return err
	}

	config := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	for _, ip := range ips {
		log.Println("Provisioning via SSH:", shellStep.Script, "in", ip)
		if err := runSSHCommand(config, net.JoinHostPort(ip, "22"), shellStep.Script, shellStep.Env); err != nil {
			return err
		}
	}

	return nil
}

func (v *Virter) VMExecRsync(ctx context.Context, copier NetworkCopier, vmNames []string, rsyncStep *ProvisionRsyncStep) error {
	files, err := filepath.Glob(rsyncStep.Source)
	if err != nil {
		return fmt.Errorf("failed to parse glob pattern: %w", err)
	}

	ips, err := v.getIPs(vmNames)
	if err != nil {
		return err
	}

	for _, ip := range ips {
		log.Printf(`Copying files via rsync: %s to %s on %s`, rsyncStep.Source, rsyncStep.Dest, ip)
		err = copier.Copy(ip, files, rsyncStep.Dest)
		if err != nil {
			return fmt.Errorf("failed to copy files: %w", err)
		}
	}
	return nil
}

func runSSHCommand(config *ssh.ClientConfig, ipPort string, script string, env []string) error {
	client, err := ssh.Dial("tcp", ipPort, config)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}

	// there is a session.Setenv(), but usually ssh, for good reasons, does only alow to set a
	// limited set of variables/no user variables at all. Fake this setting it in the script + export
	// this certainly would need some shell escaping, but hey, if a user wants to destroy her VM, have fun injecting stuff.
	var envBlob string
	for _, kv := range env {
		kvs := strings.SplitN(kv, "=", 2)
		envBlob += fmt.Sprintln(kv, "; export ", kvs[0])
	}

	inp, err := session.StdinPipe()
	if err != nil {
		return err
	}
	outp, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	errp, err := session.StderrPipe()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go logLines(&wg, "SSH stdout: ", outp)
	go logLines(&wg, "SSH stderr: ", errp)
	if err := session.Shell(); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(inp, envBlob, script); err != nil {
		return err
	}
	inp.Close()
	err = session.Wait()
	wg.Wait()

	return err
}

func (v *Virter) findVMIP(network libvirt.Network, domain libvirt.Domain) (string, error) {
	mac, err := v.getMAC(domain)
	if err != nil {
		return "", err
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

const templateMetaData = "meta-data"
const templateUserData = "user-data"
const templateCIData = "volume-cidata.xml"
const templateVMVolume = "volume-vm.xml"
const templateScratchVolume = "volume-scratch.xml"
const templateVM = "vm.xml"
