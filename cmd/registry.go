package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/LINBIT/virter/pkg/registry"
	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

const defaultRegistry = `[ubuntu-focal]
url = "https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img"

[ubuntu-bionic]
url = "https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img"

[ubuntu-xenial]
url = "https://cloud-images.ubuntu.com/xenial/current/xenial-server-cloudimg-amd64-disk1.img"

[debian-10]
url = "https://cloud.debian.org/images/cloud/buster/daily/20200119-143/debian-10-generic-amd64-daily-20200119-143.qcow2"

[debian-9]
# There is no publicly available "generic" image, but the OpenStack one works
url = "https://cdimage.debian.org/cdimage/openstack/current-9/debian-9-openstack-amd64.qcow2"

[centos-8]
url = "https://cloud.centos.org/centos/8/x86_64/images/CentOS-8-GenericCloud-8.1.1911-20200113.3.x86_64.qcow2"

[centos-7]
url = "https://cloud.centos.org/centos/7/images/CentOS-7-x86_64-GenericCloud.qcow2"

[centos-6]
url = "https://cloud.centos.org/centos/6/images/CentOS-6-x86_64-GenericCloud.qcow2"
`

func defaultRegistryPath() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := homedir.Dir()
		if err != nil {
			log.Fatal(err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "virter")
}

func userRegistryFile() string {
	return filepath.Join(configPath(), "images.toml")
}

func writeDefaultRegistry(path string) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create default registry directory ('%v'): %w",
			dir, err)
	}

	err = ioutil.WriteFile(path, []byte(defaultRegistry), 0700)
	if err != nil {
		return fmt.Errorf("failed to write default registry file: %w", err)
	}
	return nil
}

func loadRegistry() *registry.ImageRegistry {
	registryPath := filepath.Join(defaultRegistryPath(), "images.toml")
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		log.Infof("Builtin image registry does not exist, writing to %v", registryPath)
		err := writeDefaultRegistry(registryPath)
		if err != nil {
			log.Warnf("Failed to write builtin image registry file: %v", err)
			log.Warnf("Proceeding with only user-defined images")
		}
	}

	return registry.New(registryPath, userRegistryFile())
}
