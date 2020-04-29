package isogenerator

import (
	"bytes"

	"github.com/kdomanski/iso9660"
)

// ExternalISOGenerator generates ISO images using an external command
type ExternalISOGenerator struct {
}

// Generate generates a "CD-ROM" filesystem
func (g ExternalISOGenerator) Generate(files map[string][]byte) ([]byte, error) {
	isoWriter, err := iso9660.NewWriter()
	if err != nil {
		return nil, err
	}
	defer isoWriter.Cleanup()

	for name, content := range files {
		if err := isoWriter.AddFile(bytes.NewReader(content), name); err != nil {
			return nil, err
		}
	}

	wab := newWriteAtBuffer(nil)
	if err := isoWriter.WriteTo(wab, "cidata"); err != nil {
		return nil, err
	}

	return wab.Bytes(), nil
}
