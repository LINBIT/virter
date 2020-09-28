package virter_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LINBIT/containerapi"

	libvirtxml "github.com/libvirt/libvirt-go-xml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
	"github.com/LINBIT/virter/pkg/netcopy"
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
	assert.NoError(t, err)

	c.ID = 1
	_, err = virter.CheckVMConfig(c)
	assert.Error(t, err)

	c.ID = 2
	_, err = virter.CheckVMConfig(c)
	assert.NoError(t, err)
}

func TestVMRun(t *testing.T) {
	l := newFakeLibvirtConnection()

	l.vols[imageName] = &FakeLibvirtStorageVol{}

	v := virter.New(l, poolName, networkName)

	c := virter.VMConfig{
		ImageName:     imageName,
		Name:          vmName,
		ID:            vmID,
		StaticDHCP:    false,
		VCPUs:         1,
		MemoryKiB:     1024,
		SSHPublicKeys: []string{sshPublicKey},
	}
	err := v.VMRun(c)
	assert.NoError(t, err)

	assert.Empty(t, l.vols[vmName].content)

	host := l.network.description.IPs[0].DHCP.Hosts[0]
	assert.Equal(t, "52:54:00:00:00:2a", host.MAC)
	assert.Equal(t, "192.168.122.42", host.IP)

	domain := l.domains[vmName]
	assert.True(t, domain.persistent)
	assert.True(t, domain.active)
}

func TestSSHPing(t *testing.T) {
	shell := new(mocks.ShellClient)
	shell.On("DialContext", mock.Anything).Return(nil)
	shell.On("Close").Return(nil)

	sshPingConfig := virter.SSHPingConfig{
		SSHPrivateKey: []byte(sshPrivateKey),
		SSHPingCount:  1,
		SSHPingPeriod: time.Second, // ignored
	}

	l := newFakeLibvirtConnection()

	domain := newFakeLibvirtDomain(vmMAC)
	domain.active = true
	l.domains[vmName] = domain
	fakeNetworkAddHost(l.network, vmMAC, vmIP)

	v := virter.New(l, poolName, networkName)

	err := v.PingSSH(context.Background(), MockShellClientBuilder{shell}, vmName, sshPingConfig)
	assert.NoError(t, err)

	shell.AssertExpectations(t)
}

const (
	ciDataVolume     = "ciDataVolume"
	bootVolume       = "bootVolume"
	domainPersistent = "domainPersistent"
	domainCreated    = "domainCreated"
	staticDHCP       = "staticDHCP"
)

var vmRmTests = []map[string]bool{
	{},
	{
		ciDataVolume:  true,
		domainCreated: true,
	},
	{
		ciDataVolume:  true,
		bootVolume:    true,
		domainCreated: true,
	},
	{
		ciDataVolume:  true,
		bootVolume:    true,
		domainCreated: true,
	},
	{
		ciDataVolume:  true,
		bootVolume:    true,
		domainCreated: true,
	},
	{
		ciDataVolume:     true,
		bootVolume:       true,
		domainPersistent: true,
	},
	{
		ciDataVolume:     true,
		bootVolume:       true,
		domainPersistent: true,
		domainCreated:    true,
	},
	{
		ciDataVolume:     true,
		bootVolume:       true,
		domainPersistent: true,
		domainCreated:    true,
		staticDHCP:       true,
	},
}

func addDisk(l *FakeLibvirtConnection, vmName string, volumeName string) {
	disks := l.domains[vmName].description.Devices.Disks
	l.domains[vmName].description.Devices.Disks = append(disks, libvirtxml.DomainDisk{
		Source: &libvirtxml.DomainDiskSource{
			Volume: &libvirtxml.DomainDiskSourceVolume{
				Volume: ciDataVolumeName,
			},
		},
	})
}

func TestVMRm(t *testing.T) {
	for _, r := range vmRmTests {
		l := newFakeLibvirtConnection()

		if r[domainCreated] || r[domainPersistent] {
			domain := newFakeLibvirtDomain(vmMAC)
			domain.persistent = r[domainPersistent]
			domain.active = r[domainCreated]
			l.domains[vmName] = domain

			fakeNetworkAddHost(l.network, vmMAC, vmIP)
		}

		if r[bootVolume] {
			l.vols[vmName] = &FakeLibvirtStorageVol{}
		}

		if r[ciDataVolume] {
			l.vols[ciDataVolumeName] = &FakeLibvirtStorageVol{}
			addDisk(l, vmName, ciDataVolumeName)
		}

		v := virter.New(l, poolName, networkName)

		err := v.VMRm(vmName, r[staticDHCP])
		assert.NoError(t, err)

		assert.Empty(t, l.vols)
		if r[staticDHCP] {
			assert.Len(t, l.network.description.IPs[0].DHCP.Hosts, 1)
		} else {
			assert.Empty(t, l.network.description.IPs[0].DHCP.Hosts)
		}
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

		domain := newFakeLibvirtDomain(vmMAC)
		domain.persistent = true
		domain.active = r[commitDomainActive]
		l.domains[vmName] = domain

		l.vols[vmName] = &FakeLibvirtStorageVol{}
		l.vols[ciDataVolumeName] = &FakeLibvirtStorageVol{}
		addDisk(l, vmName, ciDataVolumeName)

		fakeNetworkAddHost(l.network, vmMAC, vmIP)

		an := new(mocks.AfterNotifier)

		if r[commitShutdown] {
			if r[commitShutdownTimeout] {
				timeout := make(chan time.Time, 1)
				timeout <- time.Unix(0, 0)
				mockAfter(an, timeout)
			} else {
				mockAfter(an, make(chan time.Time))
			}

		}

		v := virter.New(l, poolName, networkName)

		err := v.VMCommit(context.Background(), an, vmName, r[commitShutdown], shutdownTimeout, false)
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

func TestVMExecDocker(t *testing.T) {
	l := newFakeLibvirtConnection()

	domain := newFakeLibvirtDomain(vmMAC)
	domain.persistent = true
	domain.active = true
	l.domains[vmName] = domain

	fakeNetworkAddHost(l.network, vmMAC, vmIP)

	docker := mockContainerProvider()

	v := virter.New(l, poolName, networkName)

	containerCfg := containerapi.NewContainerConfig("test", dockerImageName, nil)
	err := v.VMExecDocker(context.Background(), docker, []string{vmName}, containerCfg, []byte(sshPrivateKey))
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

	v := virter.New(l, poolName, networkName)

	dir, err := createFakeDirectory()
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	step := &virter.ProvisionRsyncStep{
		Source: filepath.Join(dir, "*.txt"),
		Dest:   "/tmp",
	}

	copier := new(mocks.NetworkCopier)
	copier.On("Copy", mock.Anything, []netcopy.HostPath{
		{Path: filepath.Join(dir, "file1.txt")},
		{Path: filepath.Join(dir, "file2.txt")},
	}, netcopy.HostPath{Path: "/tmp", Host: "192.168.122.42"}).Return(nil)

	err = v.VMExecRsync(context.Background(), copier, []string{vmName}, step)
	assert.NoError(t, err)

	step = &virter.ProvisionRsyncStep{
		Source: filepath.Join("~/*"),
		Dest:   "/tmp",
	}
	copier2 := new(mocks.NetworkCopier)
	copierCall := copier2.On("Copy", mock.Anything, mock.AnythingOfType("[]netcopy.HostPath"), netcopy.HostPath{Path: "/tmp", Host: "192.168.122.42"}).Return(nil)
	copierCall.RunFn = func(args mock.Arguments) {
		paths := args[1].([]netcopy.HostPath)
		for _, f := range paths {
			assert.True(t, strings.HasPrefix(f.Path, os.Getenv("HOME")))
		}
	}
	err = v.VMExecRsync(context.Background(), copier2, []string{vmName}, step)
	assert.NoError(t, err)

	step = &virter.ProvisionRsyncStep{
		Source: filepath.Join("/", "323willnotbeherefile.txt"),
		Dest:   "/tmp",
	}
	copier3 := new(mocks.NetworkCopier)
	copier3.On("Copy", mock.Anything, []netcopy.HostPath{}, netcopy.HostPath{Path: "/tmp", Host: "192.168.122.42"}).Return(nil)
	err = v.VMExecRsync(context.Background(), copier3, []string{vmName}, step)
	assert.NoError(t, err)

	step = &virter.ProvisionRsyncStep{
		Source: filepath.Join(dir, "*.txt"),
		Dest:   "/tmp",
	}
	copier4 := new(mocks.NetworkCopier)
	copier4.On("Copy", mock.Anything, []netcopy.HostPath{}, netcopy.HostPath{Path: "/tmp", Host: "192.168.122.42"}).Return(nil)
	err = v.VMExecRsync(context.Background(), copier3, []string{"NoVm"}, step)
	assert.Error(t, err)
}

func TestVMExecCopy(t *testing.T) {
	l := newFakeLibvirtConnection()

	domain := newFakeLibvirtDomain(vmMAC)
	domain.persistent = true
	domain.active = true
	l.domains[vmName] = domain

	fakeNetworkAddHost(l.network, vmMAC, vmIP)

	v := virter.New(l, poolName, networkName)

	dir, err := createFakeDirectory()
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	copier := new(mocks.NetworkCopier)
	copier.On("Copy", mock.Anything, []netcopy.HostPath{
		{Path: filepath.Join(dir, "file1.txt")},
	}, netcopy.HostPath{Path: "/tmp", Host: "192.168.122.42"}).Return(nil)
	copier.On("Copy", mock.Anything, []netcopy.HostPath{
		{Path: "/tmp", Host: "192.168.122.42"},
	}, netcopy.HostPath{Path: filepath.Join(dir, "file1.txt")}).Return(nil)

	existingRemotePath := vmName + ":/tmp"
	existingLocalPathPath := filepath.Join(dir, "file1.txt")

	err = v.VMExecCopy(context.Background(), copier, []string{existingLocalPathPath}, existingRemotePath)
	assert.NoError(t, err)

	err = v.VMExecCopy(context.Background(), copier, []string{existingRemotePath}, existingLocalPathPath)
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
	vmName           = "some-vm"
	vmID             = 42
	vmMAC            = "01:23:45:67:89:ab"
	vmIP             = "192.168.122.42"
	ciDataVolumeName = vmName + "-cidata"
	sshPublicKey     = "some-key"
	shutdownTimeout  = time.Second
	dockerImageName  = "some-docker-image"
)

// A parsable private key is required; this is not authorized anywhere
const sshPrivateKey = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIN4gIrqpayWZB21fYRvZ6vMqhXRZVmeDmj2Nxg7YdGOToAoGCCqGSM49
AwEHoUQDQgAE1h7uFTDldfJM+ca8nW9dlL3zbJRXhV+g5hmm+r3ovTrvI2WA5SvS
dxX5vs9jCz9HcV6xS/2bFQXXSDxb+NHcug==
-----END EC PRIVATE KEY-----
`
