package virter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/kdomanski/iso9660"
	"github.com/kr/text"
	lx "libvirt.org/go/libvirtxml"

	"github.com/LINBIT/virter/pkg/sshkeys"
)

const templateMetaData = `instance-id: {{ .VMName }}
local-hostname: {{ .VMName }}
`

// Template used to configure DHCP clients for VMs with multiple NICs.
//
// Note: some distributions can't deal with setting some of these values to false. Instead
// we just remove them from the rendered output completely.
// Format: https://cloudinit.readthedocs.io/en/latest/topics/network-config-format-v2.html
const templateNetworkConfig = `version: 2
ethernets:
{{- range . }}
  {{ .Name }}:
{{- if .DhcpV4 }}
    dhcp4: true
{{- end }}
{{- if .DhcpV6 }}
    dhcp6: true
{{- end }}
{{- end }}
`

// The apt mirror URIs contain jinja expressions that cloud-init expands inside the guest.
// They are wrapped in Go template strings so that Go does not try to parse them.
const templateUserData = `{{- if .AptMirror -}}
## template: jinja
{{ end -}}
#cloud-config
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
{{- if .AptMirror }}
apt:
  primary:
    - arches: [default]
      uri: {{ .AptMirror }}/{{ "{{ v1.distro }}" }}
  security:
    - arches: [default]
      uri: {{ .AptMirror }}/{{ "{{ v1.distro }}" }}{% if v1.distro == 'debian' %}-security{% endif %}
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
	type NicCfg struct {
		Name   string
		DhcpV4 bool
		DhcpV6 bool
	}

	accessNet, err := v.NetworkGet(v.provisionNetwork.Name)
	if err != nil {
		return "", fmt.Errorf("failed to load access network: %w", err)
	}

	if len(nics) == 0 && !hasDhcpV6(accessNet) {
		// We know the default configuration works, so no need to make it worse in case some old cloud-init version
		// doesn't work with this type of network configuration...
		return "", nil
	}

	configuredNics := make([]NicCfg, 0, len(nics)+2)
	configuredNics = append(configuredNics,
		NicCfg{
			Name:   "eth0",
			DhcpV4: hasDhcpV4(accessNet),
			DhcpV6: hasDhcpV6(accessNet),
		},
		NicCfg{
			Name:   "enp1s0",
			DhcpV4: hasDhcpV4(accessNet),
			DhcpV6: hasDhcpV6(accessNet),
		})

	for i, nic := range nics {
		if nic.GetType() != NICTypeNetwork {
			// Extra NIC without configured dhcp support, can't enable more than the default NIC.
			return "", nil
		}

		net, err := v.NetworkGet(nic.GetSource())
		if err != nil {
			return "", fmt.Errorf("NIC assigned to unknown network: %w", err)
		}

		dhcpV4 := hasDhcpV4(net)
		dhcpV6 := hasDhcpV6(net)

		if !dhcpV4 && !dhcpV6 {
			// No DHCP configured, can't enable more than the default NIC
			return "", nil
		}

		configuredNics = append(configuredNics,
			NicCfg{
				Name:   fmt.Sprintf("eth%d", i+1),
				DhcpV4: dhcpV4,
				DhcpV6: dhcpV6,
			},
			NicCfg{
				Name:   fmt.Sprintf("enp%ds0", i+2),
				DhcpV4: dhcpV4,
				DhcpV6: dhcpV6,
			})
	}

	return renderTemplate("network-config", templateNetworkConfig, configuredNics)
}

type userDataConfig struct {
	VMName        string
	DomainSuffix  string
	SSHPublicKeys []string
	HostKey       sshkeys.HostKey
	Mounts        []string
	AptMirror     string
}

func renderUserData(cfg userDataConfig) (string, error) {
	templateData := map[string]interface{}{
		"VMName":             cfg.VMName,
		"DomainSuffix":       cfg.DomainSuffix,
		"SSHPublicKeys":      cfg.SSHPublicKeys,
		"IndentedPrivateKey": text.Indent(cfg.HostKey.PrivateKey(), "    "),
		"IndentedPublicKey":  text.Indent(cfg.HostKey.PublicKey(), "    "),
		"Mount":              cfg.Mounts,
		"AptMirror":          strings.TrimSuffix(cfg.AptMirror, "/"),
	}

	return renderTemplate("user-data", templateUserData, templateData)
}

func (v *Virter) userData(vmConfig VMConfig, sshPublicKeys []string, hostkey sshkeys.HostKey, mounts []string) (string, error) {
	domainSuffix, err := v.getDomainSuffix()
	if err != nil {
		return "", err
	}

	return renderUserData(userDataConfig{
		VMName:        vmConfig.Name,
		DomainSuffix:  domainSuffix,
		SSHPublicKeys: sshPublicKeys,
		HostKey:       hostkey,
		Mounts:        mounts,
		AptMirror:     vmConfig.AptMirror,
	})
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

	userData, err := v.userData(vmConfig, sshPublicKeys, hostkey, mounts)
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

	ciLayer, err := v.NewDynamicLayer(ciDataVolumeName(vmName), v.provisionStoragePool, WithFormat("raw"))
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

	var buf bytes.Buffer
	if err := isoWriter.WriteTo(&buf, "cidata"); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func hasDhcpV4(net *lx.Network) bool {
	for i := range net.IPs {
		if net.IPs[i].Family == "" || net.IPs[i].Family == "ipv4" {
			return net.IPs[i].DHCP != nil
		}
	}

	return false
}

func hasDhcpV6(net *lx.Network) bool {
	for i := range net.IPs {
		if net.IPs[i].Family == "ipv6" {
			return net.IPs[i].DHCP != nil
		}
	}

	return false
}
