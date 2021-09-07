package virter

// go-libvirt does not provide sufficient facilities for checking error
// responses. Use reflection instead.
//
// See https://github.com/digitalocean/go-libvirt/issues/56

import (
	"errors"

	"github.com/digitalocean/go-libvirt"
)

func hasErrorCode(err error, code libvirt.ErrorNumber) bool {
	e := libvirt.Error{}
	return errors.As(err, &e) && e.Code == uint32(code)
}
