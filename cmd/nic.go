package cmd

import (
	"github.com/LINBIT/virter/pkg/cliutils"
)

type NICArg struct {
	NicType string `arg:"type"`
	Source  string `arg:"source"`
	Model   string `arg:"model,virtio"`
	MAC     string `arg:"mac,"`
}

func (n *NICArg) GetType() string {
	return n.NicType
}

func (n *NICArg) GetSource() string {
	return n.Source
}

func (n *NICArg) GetModel() string {
	return n.Model
}

func (n *NICArg) GetMAC() string {
	return n.MAC
}

func (n *NICArg) Set(str string) error {
	return cliutils.Parse(str, n)
}

func (n *NICArg) Type() string {
	return "nic"
}
