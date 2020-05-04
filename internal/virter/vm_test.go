package virter_test

import (
	"context"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
)

func TestCheckVMConfig(t *testing.T) {
	c := virter.VMConfig{}

	_, err := virter.CheckVMConfig(c)
	assert.Error(t, err)

	c.VCPUs = 1
	_, err = virter.CheckVMConfig(c)
	assert.Error(t, err)

	c.MemoryKiB = 1024
	_, err = virter.CheckVMConfig(c)
	assert.Error(t, err)

	c.VMID = 1
	_, err = virter.CheckVMConfig(c)
	assert.Error(t, err)

	c.VMID = 2
	_, err = virter.CheckVMConfig(c)
	assert.NoError(t, err)
}

func TestVMRun(t *testing.T) {
	g := new(mocks.ISOGenerator)
	mockISOGenerate(g)

	w := new(mocks.PortWaiter)
	w.On("WaitPort", net.ParseIP("192.168.122.42"), "ssh").Return(nil)

	l := newFakeLibvirtConnection()

	l.vols[imageName] = &FakeLibvirtStorageVol{}

	v := virter.New(l, poolName, networkName, testDirectory())

	c := virter.VMConfig{
		ImageName:     imageName,
		VMName:        vmName,
		VMID:          vmID,
		VCPUs:         1,
		MemoryKiB:     1024,
		SSHPublicKeys: []string{sshPublicKey},
	}
	err := v.VMRun(g, w, c, true)
	assert.NoError(t, err)

	assert.Equal(t, []byte(ciDataContent), l.vols[ciDataVolumeName].content)
	assert.Empty(t, l.vols[vmName].content)
	assert.Empty(t, l.vols[scratchVolumeName].content)

	host := l.network.description.IPs[0].DHCP.Hosts[0]
	assert.Equal(t, "52:54:00:00:00:2a", host.MAC)
	assert.Equal(t, "192.168.122.42", host.IP)

	domain := l.domains[vmName]
	assert.True(t, domain.persistent)
	assert.True(t, domain.active)

	g.AssertExpectations(t)
	w.AssertExpectations(t)
}

func mockISOGenerate(g *mocks.ISOGenerator) {
	g.On("Generate", mock.Anything).Return([]byte(ciDataContent), nil)
}

const (
	ciDataVolume     = "ciDataVolume"
	bootVolume       = "bootVolume"
	scratchVolume    = "scratchVolume"
	domainPersistent = "domainPersistent"
	domainCreated    = "domainCreated"
)

var vmRmTests = []map[string]bool{
	{},
	{
		ciDataVolume: true,
	},
	{
		ciDataVolume: true,
		bootVolume:   true,
	},
	{
		ciDataVolume:  true,
		bootVolume:    true,
		scratchVolume: true,
	},
	{
		ciDataVolume:  true,
		bootVolume:    true,
		scratchVolume: true,
		domainCreated: true,
	},
	{
		ciDataVolume:     true,
		bootVolume:       true,
		scratchVolume:    true,
		domainPersistent: true,
	},
	{
		ciDataVolume:     true,
		bootVolume:       true,
		scratchVolume:    true,
		domainPersistent: true,
		domainCreated:    true,
	},
}

func TestVMRm(t *testing.T) {
	for _, r := range vmRmTests {
		l := newFakeLibvirtConnection()

		if r[scratchVolume] {
			l.vols[scratchVolumeName] = &FakeLibvirtStorageVol{}
		}

		if r[bootVolume] {
			l.vols[vmName] = &FakeLibvirtStorageVol{}
		}

		if r[ciDataVolume] {
			l.vols[ciDataVolumeName] = &FakeLibvirtStorageVol{}
		}

		if r[domainCreated] || r[domainPersistent] {
			domain := newFakeLibvirtDomain(vmMAC)
			domain.persistent = r[domainPersistent]
			domain.active = r[domainCreated]
			l.domains[vmName] = domain

			fakeNetworkAddHost(l.network, vmMAC, vmIP)
		}

		v := virter.New(l, poolName, networkName, testDirectory())

		err := v.VMRm(vmName)
		assert.NoError(t, err)

		assert.Empty(t, l.vols)
		assert.Empty(t, l.network.description.IPs[0].DHCP.Hosts)
		assert.Empty(t, l.domains)
	}
}

const (
	commitDomainActive    = "domainActive"
	commitShutdown        = "shutdown"
	commitShutdownTimeout = "shutdownTimeout"
)

var vmCommitTests = []map[string]bool{
	// OK: Not active
	{},
	// Error: Active, no shutdown
	{
		commitDomainActive: true,
	},
	// OK: Not active
	{
		commitShutdown: true,
	},
	// OK: Shutdown successful
	{
		commitDomainActive: true,
		commitShutdown:     true,
	},
	// Error: Shutdown timeout
	{
		commitDomainActive:    true,
		commitShutdown:        true,
		commitShutdownTimeout: true,
	},
}

func TestVMCommit(t *testing.T) {
	for _, r := range vmCommitTests {
		expectOk := !r[commitDomainActive] || (r[commitShutdown] && !r[commitShutdownTimeout])

		l := newFakeLibvirtConnection()

		l.vols[scratchVolumeName] = &FakeLibvirtStorageVol{}
		l.vols[vmName] = &FakeLibvirtStorageVol{}
		l.vols[ciDataVolumeName] = &FakeLibvirtStorageVol{}

		domain := newFakeLibvirtDomain(vmMAC)
		domain.persistent = true
		domain.active = r[commitDomainActive]
		l.domains[vmName] = domain

		fakeNetworkAddHost(l.network, vmMAC, vmIP)

		an := new(mocks.AfterNotifier)

		if r[commitShutdown] {
			if r[commitShutdownTimeout] {
				l.lifecycleEvents = make(chan libvirt.DomainEventLifecycleMsg)
				timeout := make(chan time.Time, 1)
				timeout <- time.Unix(0, 0)
				mockAfter(an, timeout)
			} else {
				l.lifecycleEvents = makeShutdownEvents()
				mockAfter(an, make(chan time.Time))
			}

		}

		v := virter.New(l, poolName, networkName, testDirectory())

		err := v.VMCommit(an, vmName, r[commitShutdown], shutdownTimeout)
		if expectOk {
			assert.NoError(t, err)

			assert.Len(t, l.vols, 1)
			assert.Empty(t, l.network.description.IPs[0].DHCP.Hosts)
			assert.Empty(t, l.domains)
		} else {
			assert.Error(t, err)
		}

		an.AssertExpectations(t)
	}
}

func makeShutdownEvents() chan libvirt.DomainEventLifecycleMsg {
	events := make(chan libvirt.DomainEventLifecycleMsg, 1)
	events <- libvirt.DomainEventLifecycleMsg{
		Dom:    libvirt.Domain{Name: vmName},
		Event:  int32(libvirt.DomainEventStopped),
		Detail: int32(libvirt.DomainEventStoppedShutdown),
	}
	return events
}

func TestVMExecDocker(t *testing.T) {
	l := newFakeLibvirtConnection()

	domain := newFakeLibvirtDomain(vmMAC)
	domain.persistent = true
	domain.active = true
	l.domains[vmName] = domain

	fakeNetworkAddHost(l.network, vmMAC, vmIP)

	docker := new(mocks.DockerClient)
	mockDockerRun(docker)

	v := virter.New(l, poolName, networkName, testDirectory())

	dockercfg := virter.DockerContainerConfig{ImageName: dockerImageName}
	err := v.VMExecDocker(context.Background(), docker, []string{vmName}, dockercfg, []byte(sshPrivateKey))
	assert.NoError(t, err)

	docker.AssertExpectations(t)
}

func TestVMExecRsync(t *testing.T) {
	l := newFakeLibvirtConnection()

	domain := newFakeLibvirtDomain(vmMAC)
	domain.persistent = true
	domain.active = true
	l.domains[vmName] = domain

	fakeNetworkAddHost(l.network, vmMAC, vmIP)

	v := virter.New(l, poolName, networkName, testDirectory())

	dir, err := createFakeDirectory()
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	step := &virter.ProvisionRsyncStep{
		Source: filepath.Join(dir, "*.txt"),
		Dest:   "/tmp",
	}

	copier := new(mocks.NetworkCopier)
	copier.On("Copy", "192.168.122.42", []string{
		filepath.Join(dir, "file1.txt"),
		filepath.Join(dir, "file2.txt"),
	}, "/tmp").Return(nil)

	err = v.VMExecRsync(context.Background(), copier, []string{vmName}, step)
	assert.NoError(t, err)

	step = &virter.ProvisionRsyncStep{
		Source: filepath.Join("~/*"),
		Dest:   "/tmp",
	}
	copier2 := new(mocks.NetworkCopier)
	copierCall := copier2.On("Copy", "192.168.122.42", mock.AnythingOfType("[]string"), "/tmp").Return(nil)
	copierCall.RunFn = func(args mock.Arguments) {
		files := args[1].([]string)
		for _, f := range files {
			assert.True(t, strings.HasPrefix(f, os.Getenv("HOME")))
		}
	}
	err = v.VMExecRsync(context.Background(), copier2, []string{vmName}, step)
	assert.NoError(t, err)

	step = &virter.ProvisionRsyncStep{
		Source: filepath.Join("/", "323willnotbeherefile.txt"),
		Dest:   "/tmp",
	}
	copier3 := new(mocks.NetworkCopier)
	copier3.AssertNotCalled(t, "Copy")
	err = v.VMExecRsync(context.Background(), copier3, []string{vmName}, step)
	assert.NoError(t, err)
}

func createFakeDirectory() (string, error) {
	dir, err := ioutil.TempDir("/tmp", "virter-test")
	if err != nil {
		return "", err
	}

	if _, err := os.Create(filepath.Join(dir, "file1.txt")); err != nil {
		return "", err
	}
	if _, err := os.Create(filepath.Join(dir, "file1.go")); err != nil {
		return "", err
	}
	if _, err := os.Create(filepath.Join(dir, "file2.txt")); err != nil {
		return "", err
	}
	return dir, nil
}

func mockAfter(an *mocks.AfterNotifier, timeout <-chan time.Time) {
	an.On("After", shutdownTimeout).Return(timeout)
}

const (
	vmName            = "some-vm"
	vmID              = 42
	vmMAC             = "01:23:45:67:89:ab"
	vmIP              = "192.168.122.42"
	ciDataVolumeName  = vmName + "-cidata"
	scratchVolumeName = vmName + "-scratch"
	ciDataContent     = "some-ci-data"
	sshPublicKey      = "some-key"
	sshPrivateKey     = "some-private-key"
	shutdownTimeout   = time.Second
	dockerImageName   = "some-docker-image"
)
