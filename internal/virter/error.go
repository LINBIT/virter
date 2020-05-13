package virter

// go-libvirt does not provide sufficient facilities for checking error
// responses. Use reflection instead.
//
// See https://github.com/digitalocean/go-libvirt/issues/56

import (
	"reflect"
)

type errorNumber int32

const (
	errNoDomain     errorNumber = 42
	errNoStorageVol errorNumber = 50
)

func hasErrorCode(err error, code errorNumber) bool {
	v := reflect.ValueOf(err)
	if v.Kind() != reflect.Struct {
		return false
	}

	c := v.FieldByName("Code")
	if c.Kind() != reflect.Uint32 {
		return false
	}

	return c.Uint() == uint64(code)
}
