package virter

import (
	"fmt"
	"runtime"
	"strings"

	lx "github.com/libvirt/libvirt-go-xml"
)

type CpuArch string

const (
	CpuArchAMD64   = CpuArch("amd64")
	CpuArchARM64   = CpuArch("arm64")
	CpuArchPPC64LE = CpuArch("ppc64le")
	CpuArchNative  = CpuArch(runtime.GOARCH)
)

func (c *CpuArch) String() string {
	arch := c.get()
	return string(arch)
}

func (c *CpuArch) Set(s string) error {
	switch CpuArch(strings.ToLower(s)) {
	case CpuArchAMD64:
		*c = CpuArchAMD64
	case CpuArchARM64:
		*c = CpuArchARM64
	case CpuArchPPC64LE:
		*c = CpuArchPPC64LE
	case "":
		*c = CpuArchNative
	default:
		return unknownArch(s)
	}

	return nil
}

func (c *CpuArch) Type() string {
	return "arch"
}

type unknownArch string

func (u unknownArch) Error() string {
	return fmt.Sprintf("unknown arch '%s', supported are: %+v", string(u), []CpuArch{CpuArchAMD64, CpuArchARM64, CpuArchPPC64LE})
}

func (c *CpuArch) DomainType() string {
	arch := c.get()

	if arch == CpuArchNative {
		return "kvm"
	}

	return "qemu"
}

func (c *CpuArch) QemuArch() string {
	arch := c.get()

	switch arch {
	case CpuArchAMD64:
		return "x86_64"
	case CpuArchARM64:
		return "aarch64"
	case CpuArchPPC64LE:
		return "ppc64"
	default:
		return ""
	}
}

func (c *CpuArch) OSDomain() *lx.DomainOS {
	return &lx.DomainOS{
		Type: &lx.DomainOSType{
			Arch:    c.QemuArch(),
			Type:    "hvm",
			Machine: c.Machine(),
		},
		Firmware:    c.Firmware(),
		BootDevices: []lx.DomainBootDevice{{Dev: "hd"}},
	}
}

func (c *CpuArch) Firmware() string {
	arch := c.get()

	switch arch {
	case CpuArchARM64:
		return "efi"
	default:
		return ""
	}
}

func (c *CpuArch) CPU() *lx.DomainCPU {
	arch := c.get()

	if arch == CpuArchNative {
		return &lx.DomainCPU{
			Mode: "host-model",
		}
	}

	switch arch {
	case CpuArchAMD64:
		return &lx.DomainCPU{
			Mode:  "custom",
			Match: "exact",
			Model: &lx.DomainCPUModel{
				Value:    "max",
				Fallback: "forbid",
			},
		}
	case CpuArchARM64:
		return &lx.DomainCPU{
			Mode:  "custom",
			Match: "exact",
			Model: &lx.DomainCPUModel{
				Value:    "cortex-a72",
				Fallback: "forbid",
			},
		}
	case CpuArchPPC64LE:
		return &lx.DomainCPU{
			Mode:  "custom",
			Match: "exact",
			Model: &lx.DomainCPUModel{
				Value:    "power10",
				Fallback: "forbid",
			},
		}
	default:
		return nil
	}
}

func (c *CpuArch) Machine() string {
	switch c.get() {
	case CpuArchAMD64:
		return "q35"
	case CpuArchARM64:
		return "virt"
	case CpuArchPPC64LE:
		return "pseries"
	default:
		return ""
	}
}

func (c *CpuArch) PM() *lx.DomainPM {
	arch := c.get()

	switch arch {
	case CpuArchAMD64:
		return &lx.DomainPM{
			SuspendToDisk: &lx.DomainPMPolicy{Enabled: "no"},
			SuspendToMem:  &lx.DomainPMPolicy{Enabled: "no"},
		}
	default:
		return nil
	}
}

func (c *CpuArch) get() CpuArch {
	if c == nil || *c == "" {
		return CpuArchNative
	}

	return *c
}
