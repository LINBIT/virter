package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/digitalocean/go-libvirt"

	"github.com/LINBIT/virter/internal"
)

func main() {
	err := imagePull()
	if err != nil {
		log.Println(err)
	}
}

func imagePull() error {
	c, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 2*time.Second)
	if err != nil {
		return fmt.Errorf("failed to dial libvirt: %w", err)
	}

	l := libvirt.New(c)
	if err := l.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	client := &http.Client{}

	internal.ImagePull(l, client, "http://example.com")
	return nil
}
