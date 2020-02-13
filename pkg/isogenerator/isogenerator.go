package isogenerator

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

// ExternalISOGenerator generates ISO images using an external command
type ExternalISOGenerator struct {
}

// Generate generates a "CD-ROM" filesystem
func (g ExternalISOGenerator) Generate(files map[string][]byte) ([]byte, error) {
	dir, err := ioutil.TempDir(os.TempDir(), "virter-iso-root")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(dir)

	var filePaths []string

	for name, content := range files {
		path := filepath.Join(dir, name)
		ioutil.WriteFile(path, content, 0600)
		filePaths = append(filePaths, path)
	}

	args := []string{
		"-input-charset", "utf-8",
		"-volid", "cidata",
		"-joliet",
		"-rock"}

	args = append(args, filePaths...)

	return exec.Command("genisoimage", args...).Output()
}
