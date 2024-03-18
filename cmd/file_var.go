package cmd

import (
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

var _ pflag.Value = &FileVar{}
