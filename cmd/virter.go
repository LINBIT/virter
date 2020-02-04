package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/digitalocean/go-libvirt"

	"github.com/LINBIT/virter/internal"
	"github.com/LINBIT/virter/pkg/directory"
)

func main() {
	err := imagePull()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func imagePull() error {
	var templates directory.Directory = "assets/libvirt-templates"

	c, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 2*time.Second)
	if err != nil {
		return fmt.Errorf("failed to dial libvirt: %w", err)
	}

	l := libvirt.New(c)
	if err := l.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	v := internal.New(l, templates)

	client := &http.Client{}

	return v.ImagePull(client, "http://example.com")
}
