package virter

import (
	"fmt"
	"runtime"
	"strings"

	lx "libvirt.org/go/libvirtxml"
)

type CpuArch string

const (
	CpuArchAMD64   = CpuArch("amd64")
	CpuArchARM64   = CpuArch("arm64")
	CpuArchPPC64LE = CpuArch("ppc64le")
	CpuArchS390x   = CpuArch("s390x")
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
	case CpuArchS390x:
		*c = CpuArchS390x
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

type CpuMode string

const (
	CpuModeHostModel       = CpuMode("host-model")
	CpuModeHostPassthrough = CpuMode("host-passthrough")
)

func (m *CpuMode) String() string {
	return string(*m)
}

func (m *CpuMode) Set(s string) error {
	switch CpuMode(strings.ToLower(s)) {
	case CpuModeHostModel:
		*m = CpuModeHostModel
	case CpuModeHostPassthrough:
		*m = CpuModeHostPassthrough
	case "":
		*m = ""
	default:
		return fmt.Errorf("unknown CPU mode '%s', supported are: %+v", s, []CpuMode{CpuModeHostModel, CpuModeHostPassthrough})
	}

	return nil
}

func (m *CpuMode) Type() string {
	return "cpu-mode"
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
	case CpuArchS390x:
		return "s390x"
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

func (c *CpuArch) CPU(mode CpuMode, model string, nestedVirtFeature string) *lx.DomainCPU {
	cpu := c.defaultCPU()

	if model != "" {
		cpu = &lx.DomainCPU{
			Mode:  "custom",
			Match: "exact",
			Model: &lx.DomainCPUModel{
				Value:    model,
				Fallback: "forbid",
			},
		}
	} else if mode != "" {
		cpu = &lx.DomainCPU{
			Mode: string(mode),
		}
	}

	if cpu != nil && nestedVirtFeature != "" {
		cpu.Features = append(cpu.Features, lx.DomainCPUFeature{
			Policy: "require",
			Name:   nestedVirtFeature,
		})
	}

	return cpu
}

// NestedVirtFeature maps the host CPU vendor to the CPU feature enabling nested virtualization.
func NestedVirtFeature(domCapsXML string) (string, error) {
	caps := lx.DomainCaps{}
	if err := caps.Unmarshal(domCapsXML); err != nil {
		return "", fmt.Errorf("failed to parse domain capabilities: %w", err)
	}

	vendor := ""
	if caps.CPU != nil {
		for _, m := range caps.CPU.Modes {
			if m.Name == "host-model" {
				vendor = m.Vendor
			}
		}
	}

	switch vendor {
	case "AMD":
		return "svm", nil
	case "Intel":
		return "vmx", nil
	default:
		return "", fmt.Errorf("nested virtualization is not supported for host CPU vendor '%s'", vendor)
	}
}

func (c *CpuArch) defaultCPU() *lx.DomainCPU {
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
	case CpuArchS390x:
		return &lx.DomainCPU{
			Mode:  "custom",
			Match: "exact",
			Model: &lx.DomainCPUModel{
				Value:    "max",
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
	case CpuArchS390x:
		return "s390-ccw-virtio"
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
