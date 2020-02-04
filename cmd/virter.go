package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/libvirt/libvirt-go"

	"github.com/LINBIT/virter/internal"
	"github.com/LINBIT/virter/internal/libvirtinterfaces"
)

type LibvirtConnect struct {
	libvirt.Connect
}

func (c *LibvirtConnect) LookupStoragePoolByName(name string) (libvirtinterfaces.LibvirtStoragePool, error) {
	return c.Connect.LookupStoragePoolByName(name)
}

func (c *LibvirtConnect) NewStream(flags libvirt.StreamFlags) (libvirtinterfaces.LibvirtStream, error) {
	return c.Connect.NewStream(flags)
}

func main() {
	err := imagePull()
	if err != nil {
		log.Println(err)
	}
}

func imagePull() error {
	connect, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return fmt.Errorf("could not connect to hypervisor: %w", err)
	}

	connect2 := &LibvirtConnect{*connect}

	client := &http.Client{}

	internal.ImagePull(connect2, client, "http://example.com")
	return nil
}
