package directory

import (
	"io/ioutil"
	"path/filepath"
)

// Directory is a directory on the filesystem.
type Directory string

// ReadFile reads a file relative to a directory.
func (d Directory) ReadFile(subpath string) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(string(d), subpath))
}
