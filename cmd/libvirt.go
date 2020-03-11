package cmd

import (
	"fmt"
	"net"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/directory"
)

// VirterConnect connects to a local libvirt instance
func VirterConnect() (*virter.Virter, error) {
	var templates directory.Directory = directory.Directory(viper.GetString("libvirt.template_dir"))

	c, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to dial libvirt: %w", err)
	}

	l := libvirt.New(c)
	if err := l.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	pool := viper.GetString("libvirt.pool")

	return virter.New(l, pool, templates), nil
}
