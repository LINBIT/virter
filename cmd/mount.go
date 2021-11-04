package cmd

import "github.com/LINBIT/virter/pkg/cliutils"

type MountArg struct {
	HostPath string `arg:"host"`
	VmPath   string `arg:"vm"`
}

func (s *MountArg) GetHostPath() string {
	return s.HostPath
}

func (s *MountArg) GetVMPath() string {
	return s.VmPath
}

func (s *MountArg) Set(str string) error {
	return cliutils.Parse(str, s)
}

func (s *MountArg) Type() string {
	return "mount"
}
