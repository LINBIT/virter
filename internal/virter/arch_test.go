package virter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	lx "libvirt.org/go/libvirtxml"

	"github.com/LINBIT/virter/internal/virter"
)

func TestCpuModeSet(t *testing.T) {
	cases := []struct {
		input    string
		expected virter.CpuMode
		valid    bool
	}{
		{input: "host-model", expected: virter.CpuModeHostModel, valid: true},
		{input: "host-passthrough", expected: virter.CpuModeHostPassthrough, valid: true},
		{input: "HOST-PASSTHROUGH", expected: virter.CpuModeHostPassthrough, valid: true},
		{input: "", expected: virter.CpuMode(""), valid: true},
		{input: "custom", valid: false},
		{input: "EPYC-Milan", valid: false},
	}

	for _, c := range cases {
		var m virter.CpuMode
		err := m.Set(c.input)
		if c.valid {
			assert.NoError(t, err, "input %q", c.input)
			assert.Equal(t, c.expected, m, "input %q", c.input)
		} else {
			assert.Error(t, err, "input %q", c.input)
		}
	}
}

func TestCpuArchCPU(t *testing.T) {
	native := virter.CpuArchNative

	// default: unchanged behavior for the native architecture
	cpu := native.CPU("", "", "")
	assert.Equal(t, "host-model", cpu.Mode)
	assert.Nil(t, cpu.Model)
	assert.Empty(t, cpu.Features)

	// mode override
	cpu = native.CPU(virter.CpuModeHostPassthrough, "", "")
	assert.Equal(t, "host-passthrough", cpu.Mode)
	assert.Nil(t, cpu.Model)
	assert.Empty(t, cpu.Features)

	// model pin implies mode "custom"
	cpu = native.CPU("", "EPYC-Milan", "")
	assert.Equal(t, "custom", cpu.Mode)
	assert.Equal(t, "exact", cpu.Match)
	assert.Equal(t, &lx.DomainCPUModel{Value: "EPYC-Milan", Fallback: "forbid"}, cpu.Model)
	assert.Empty(t, cpu.Features)

	// nested virtualization feature is appended to any mode
	cpu = native.CPU("", "", "svm")
	assert.Equal(t, "host-model", cpu.Mode)
	assert.Equal(t, []lx.DomainCPUFeature{{Policy: "require", Name: "svm"}}, cpu.Features)

	cpu = native.CPU(virter.CpuModeHostPassthrough, "", "vmx")
	assert.Equal(t, "host-passthrough", cpu.Mode)
	assert.Equal(t, []lx.DomainCPUFeature{{Policy: "require", Name: "vmx"}}, cpu.Features)

	// model pin replaces the default model of an emulated architecture too
	other := virter.CpuArchARM64
	defaultModel := "cortex-a72"
	if other == virter.CpuArchNative {
		other = virter.CpuArchAMD64
		defaultModel = "max"
	}
	cpu = other.CPU("", "", "")
	assert.Equal(t, "custom", cpu.Mode)
	assert.Equal(t, defaultModel, cpu.Model.Value)

	cpu = other.CPU("", "cortex-a76", "")
	assert.Equal(t, "custom", cpu.Mode)
	assert.Equal(t, "cortex-a76", cpu.Model.Value)
}

func TestNestedVirtFeature(t *testing.T) {
	feature, err := virter.NestedVirtFeature(`<domainCapabilities>
  <path>/usr/bin/qemu-system-x86_64</path>
  <domain>kvm</domain>
  <arch>x86_64</arch>
  <cpu>
    <mode name='host-model' supported='yes'>
      <model fallback='forbid'>EPYC-Milan</model>
      <vendor>AMD</vendor>
    </mode>
  </cpu>
</domainCapabilities>`)
	assert.NoError(t, err)
	assert.Equal(t, "svm", feature)

	feature, err = virter.NestedVirtFeature(`<domainCapabilities>
  <path>/usr/bin/qemu-system-x86_64</path>
  <domain>kvm</domain>
  <arch>x86_64</arch>
  <cpu>
    <mode name='host-model' supported='yes'>
      <model fallback='forbid'>Skylake-Client</model>
      <vendor>Intel</vendor>
    </mode>
  </cpu>
</domainCapabilities>`)
	assert.NoError(t, err)
	assert.Equal(t, "vmx", feature)

	// no vendor information
	_, err = virter.NestedVirtFeature(`<domainCapabilities>
  <path>/usr/bin/qemu-system-aarch64</path>
  <domain>qemu</domain>
  <arch>aarch64</arch>
  <cpu>
    <mode name='host-model' supported='no'/>
  </cpu>
</domainCapabilities>`)
	assert.Error(t, err)

	// invalid XML
	_, err = virter.NestedVirtFeature("not xml")
	assert.Error(t, err)
}
