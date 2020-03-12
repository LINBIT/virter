package virter

import (
	"fmt"

	"github.com/digitalocean/go-libvirt"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
)

func getNetworkDescription(l LibvirtConnection, network libvirt.Network) (*libvirtxml.Network, error) {
	var description libvirtxml.Network

	networkXML, err := l.NetworkGetXMLDesc(network, 0)
	if err != nil {
		return &description, fmt.Errorf("could not get network XML: %w", err)
	}

	err = description.Unmarshal(networkXML)
	if err != nil {
		return &description, fmt.Errorf("could not unmarshal network XML: %w", err)
	}

	return &description, nil
}

func getDomainDescription(l LibvirtConnection, domain libvirt.Domain) (*libvirtxml.Domain, error) {
	var description libvirtxml.Domain

	domainXML, err := l.DomainGetXMLDesc(domain, 0)
	if err != nil {
		return &description, fmt.Errorf("could not get domain XML: %w", err)
	}

	err = description.Unmarshal(domainXML)
	if err != nil {
		return &description, fmt.Errorf("could not unmarshal domain XML: %w", err)
	}

	return &description, nil
}
