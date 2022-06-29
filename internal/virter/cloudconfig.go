package virter

import (
	"bytes"
	"fmt"

	"github.com/kdomanski/iso9660"
	"github.com/kr/text"
	lx "github.com/libvirt/libvirt-go-xml"

	"github.com/LINBIT/virter/pkg/sshkeys"
)

const templateMetaData = `instance-id: {{ .VMName }}
local-hostname: {{ .VMName }}
`

const templateNetworkConfig = `version: 2
ethernets:
{{- range . }}
  {{ . }}:
    dhcp4: true
{{- end }}
`

const templateUserData = `#cloud-config
disable_root: False
# Ideally, we would set this to "unchanged". However, this causes cloud-init on centos-6
# to produce an invalid SSHd config, completely preventing external access to the VM.
ssh_pwauth: True
ssh_authorized_keys:
{{- range .SSHPublicKeys }}
  - {{ . }}
{{- end }}
ssh_keys:
  rsa_private: |
{{ .IndentedPrivateKey }}
  rsa_public: |
{{ .IndentedPublicKey }}
preserve_hostname: false
hostname: {{ .VMName }}
{{- if .DomainSuffix }}
fqdn: {{ .VMName }}.{{ .DomainSuffix }}
{{- else }}
fqdn: {{ .VMName }}
{{- end }}
{{- if .Mount }}
mounts:
{{- range .Mount }}
  - [ "{{ . }}", "{{ . }}", "virtiofs"]
{{- end }}
{{- end }}
`

func (v *Virter) metaData(vmName string) (string, error) {
	templateData := map[string]interface{}{
		"VMName": vmName,
	}

	return renderTemplate("meta-data", templateMetaData, templateData)
}

// NetworkConfig returns cloud-init configuration, initializing all networks with DHCP if possible.
//
// See the end of ./doc/networks.md for limitations.
func (v *Virter) NetworkConfig(nics []NIC) (string, error) {
	if len(nics) == 0 {
		// We know the default configuration works, so no need to make it worse in case some old cloud-init version
		// doesn't work with this type of network configuration...
		return "", nil
	}

	configuredNics := make([]string, 0, len(nics)+2)
	configuredNics = append(configuredNics, "eth0", "enp1s0")

	allNets, err := v.NetworkList()
	if err != nil {
		return "", err
	}

	for i, nic := range nics {
		if nic.GetType() != NICTypeNetwork {
			// Extra NIC without configured dhcp support, can't enable more than the default NIC.
			return "", nil
		}

		var net *lx.Network
		for i := range allNets {
			if allNets[i].Name == nic.GetSource() {
				net = &allNets[i]
			}
		}

		if net == nil {
			return "", fmt.Errorf("NIC assigned to unknown network '%s'", nic.GetSource())
		}

		if len(net.IPs) < 1 {
			// No IPs configure -> no DHCP configured, can't enable more than the default NIC
			return "", nil
		}

		if net.IPs[0].DHCP == nil {
			// No DHCP configured, can't enable more than the default NIC
			return "", nil
		}

		configuredNics = append(configuredNics, fmt.Sprintf("eth%d", i+1), fmt.Sprintf("enp%ds0", i+2))
	}

	return renderTemplate("network-config", templateNetworkConfig, configuredNics)
}

func (v *Virter) userData(vmName string, sshPublicKeys []string, hostkey sshkeys.HostKey, mounts []string) (string, error) {
	privateKey := text.Indent(hostkey.PrivateKey(), "    ")
	publicKey := text.Indent(hostkey.PublicKey(), "    ")

	domainSuffix, err := v.getDomainSuffix()
	if err != nil {
		return "", err
	}

	templateData := map[string]interface{}{
		"VMName":             vmName,
		"DomainSuffix":       domainSuffix,
		"SSHPublicKeys":      sshPublicKeys,
		"IndentedPrivateKey": privateKey,
		"IndentedPublicKey":  publicKey,
		"Mount":              mounts,
	}

	return renderTemplate("user-data", templateUserData, templateData)
}

func (v *Virter) createCIData(vmConfig VMConfig, hostkey sshkeys.HostKey) (*RawLayer, error) {
	vmName := vmConfig.Name
	sshPublicKeys := append(vmConfig.ExtraSSHPublicKeys, string(v.sshkeys.PublicKey()))

	metaData, err := v.metaData(vmName)
	if err != nil {
		return nil, err
	}

	networkConfig, err := v.NetworkConfig(vmConfig.ExtraNics)
	if err != nil {
		return nil, err
	}

	mounts := make([]string, len(vmConfig.Mounts))
	for i, m := range vmConfig.Mounts {
		mounts[i] = m.GetVMPath()
	}

	userData, err := v.userData(vmName, sshPublicKeys, hostkey, mounts)
	if err != nil {
		return nil, err
	}

	files := map[string][]byte{
		"meta-data": []byte(metaData),
		"user-data": []byte(userData),
	}

	// Only explicitly add network config if we have something to configure.
	// Otherwise, cloud-init might not configure the network at all.
	if networkConfig != "" {
		files["network-config"] = []byte(networkConfig)
	}

	ciData, err := GenerateISO(files)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ISO: %w", err)
	}

	ciLayer, err := v.NewDynamicLayer(ciDataVolumeName(vmName), WithFormat("raw"))
	if err != nil {
		return nil, err
	}

	err = ciLayer.Upload(bytes.NewReader(ciData))
	if err != nil {
		return nil, fmt.Errorf("failed to transfer cloud-init data to libvirt: %w", err)
	}

	return ciLayer, nil
}

func ciDataVolumeName(vmName string) string {
	return vmName + "-cidata"
}

// GenerateISO generates a "CD-ROM" filesystem
func GenerateISO(files map[string][]byte) ([]byte, error) {
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
