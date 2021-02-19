package virter

import (
	"bytes"
	"fmt"
	"github.com/LINBIT/virter/pkg/sshkeys"
	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/kdomanski/iso9660"
	"github.com/kr/text"
)

const templateMetaData = `instance-id: {{ .VMName }}
local-hostname: {{ .VMName }}
`

const templateUserData = `#cloud-config
disable_root: False
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
`

func (v *Virter) metaData(vmName string) (string, error) {
	templateData := map[string]interface{}{
		"VMName": vmName,
	}

	return renderTemplate("meta-data", templateMetaData, templateData)
}

func (v *Virter) userData(vmName string, sshPublicKeys []string, hostkey sshkeys.HostKey) (string, error) {
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
	}

	return renderTemplate("user-data", templateUserData, templateData)
}

func (v *Virter) createCIData(sp libvirt.StoragePool, vmConfig VMConfig, hostkey sshkeys.HostKey) error {
	vmName := vmConfig.Name
	sshPublicKeys := append(vmConfig.ExtraSSHPublicKeys, string(v.sshkeys.PublicKey()))

	metaData, err := v.metaData(vmName)
	if err != nil {
		return err
	}

	userData, err := v.userData(vmName, sshPublicKeys, hostkey)
	if err != nil {
		return err
	}

	files := map[string][]byte{
		"meta-data": []byte(metaData),
		"user-data": []byte(userData),
	}

	ciData, err := GenerateISO(files)
	if err != nil {
		return fmt.Errorf("failed to generate ISO: %w", err)
	}

	xml, err := v.ciDataVolumeXML(ciDataVolumeName(vmName))
	if err != nil {
		return err
	}

	sv, err := v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create cloud-init volume: %w", err)
	}

	err = v.libvirt.StorageVolUpload(sv, bytes.NewReader(ciData), 0, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to transfer cloud-init data to libvirt: %w", err)
	}

	return nil
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
