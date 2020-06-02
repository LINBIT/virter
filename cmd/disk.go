package cmd

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/rck/unit"
	log "github.com/sirupsen/logrus"
)

// DiskArg represents a disk that can be passed to virter via a command line argument.
type DiskArg struct {
	Name    string `key:"name" required:"true"`
	SizeKiB uint64 `key:"size" required:"true"`
	Format  string `key:"format" default:"qcow2"`
	Bus     string `key:"bus" default:"virtio"`
}

func (d *DiskArg) GetName() string    { return d.Name }
func (d *DiskArg) GetSizeKiB() uint64 { return d.SizeKiB }
func (d *DiskArg) GetFormat() string  { return d.Format }
func (d *DiskArg) GetBus() string     { return d.Bus }

func parseArgMap(str string) (map[string]string, error) {
	result := map[string]string{}

	kvpairs := strings.Split(str, ",")
	for _, kvpair := range kvpairs {
		if kvpair == "" {
			continue
		}
		kv := strings.Split(kvpair, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("malformed key/value pair '%s': expected exactly one '='", kvpair)
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		if key == "" {
			return nil, fmt.Errorf("malformed key/value pair '%s': key cannot be empty", kvpair)
		}

		result[key] = value
	}

	return result, nil
}

func fillDefaultValues(p map[string]string) (map[string]string, error) {
	t := reflect.TypeOf(DiskArg{})
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		key := field.Tag.Get("key")
		required := field.Tag.Get("required")
		defaultValue := field.Tag.Get("default")

		if _, ok := p[key]; !ok {
			if required == "true" {
				return nil, fmt.Errorf("missing required parameter '%s'", key)
			} else {
				p[key] = defaultValue
			}
		}
	}

	return p, nil
}

// Set implements flag.Value.Set.
func (d *DiskArg) Set(str string) error {
	if len(str) == 0 {
		return fmt.Errorf("invalid empty disk specification")
	}

	params, err := parseArgMap(str)
	if err != nil {
		return fmt.Errorf("failed to parse disk specification: %w", err)
	}

	params, err = fillDefaultValues(params)
	if err != nil {
		return fmt.Errorf("failed to parse disk specification: %w", err)
	}

	for k, v := range params {
		switch k {
		case "name":
			d.Name = v
		case "size":
			u := unit.MustNewUnit(sizeUnits)
			val, err := u.ValueFromString(v)
			if err != nil {
				return fmt.Errorf("invalid size: %w", err)
			}
			signedSizeKiB := val.Value / sizeUnits["K"]
			if signedSizeKiB < 0 {
				return fmt.Errorf("invalid size: must be positive number")
			}
			d.SizeKiB = uint64(signedSizeKiB)
		case "format":
			d.Format = v
		case "bus":
			d.Bus = v
		default:
			log.Debugf("ignoring unknown disk key: %v", k)
		}
	}
	return nil
}

// Type implements pflag.Value.Type.
func (d *DiskArg) Type() string { return "disk" }
