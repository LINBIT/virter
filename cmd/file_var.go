package cmd

import (
	"io"
	"os"

	"github.com/spf13/pflag"
)

type FileVar struct {
	File *os.File
}

func (f *FileVar) Type() string {
	return "file"
}

func (f *FileVar) String() string {
	if f.File == nil {
		return ""
	}
	return f.File.Name()
}

func (f *FileVar) Set(s string) error {
	fd, err := os.Open(s)
	if err != nil {
		return err
	}

	f.File = fd
	return nil
}

// Reader returns the opened file, or nil if no file was given. Use this rather
// than File where an io.Reader is expected, because a nil *os.File in an
// interface is not equal to nil.
func (f *FileVar) Reader() io.ReadCloser {
	if f.File == nil {
		return nil
	}
	return f.File
}

var _ pflag.Value = &FileVar{}
