package virter

import (
	"fmt"

	"github.com/digitalocean/go-libvirt"
	libvirtxml "github.com/libvirt/libvirt-go-xml"
)

type LibvirtGetError struct {
	Message  string
	Err      error
	NotFound bool
}

func (e *LibvirtGetError) Error() string {
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *LibvirtGetError) Unwrap() error {
	return e.Err
}

func getNetworkDescription(l LibvirtConnection, network libvirt.Network) (*libvirtxml.Network, error) {
	var description libvirtxml.Network

	networkXML, err := l.NetworkGetXMLDesc(network, 0)
	if err != nil {
		return &description, &LibvirtGetError{Message: "could not get network XML", Err: err, NotFound: hasErrorCode(err, libvirt.ErrNoNetwork)}
	}

	err = description.Unmarshal(networkXML)
	if err != nil {
		return &description, &LibvirtGetError{Message: "could not unmarshal network XML", Err: err}
	}

	return &description, nil
}

func getDomainDescription(l LibvirtConnection, domain libvirt.Domain) (*libvirtxml.Domain, error) {
	var description libvirtxml.Domain

	domainXML, err := l.DomainGetXMLDesc(domain, 0)
	if err != nil {
		return &description, &LibvirtGetError{Message: "could not get domain XML", Err: err, NotFound: hasErrorCode(err, libvirt.ErrNoDomain)}
	}

	err = description.Unmarshal(domainXML)
	if err != nil {
		return &description, &LibvirtGetError{Message: "could not unmarshal domain XML", Err: err}
	}

	return &description, nil
}
