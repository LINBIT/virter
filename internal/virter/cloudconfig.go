package virter

import (
	"bytes"
	"fmt"

	"github.com/digitalocean/go-libvirt"
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
preserve_hostname: false
hostname: {{ .VMName }}
fqdn: {{ .VMName }}.test
`

func (v *Virter) metaData(vmName string) (string, error) {
	templateData := map[string]interface{}{
		"VMName": vmName,
	}

	return renderTemplate("meta-data", templateMetaData, templateData)
}

func (v *Virter) userData(vmName string, sshPublicKeys []string) (string, error) {
	templateData := map[string]interface{}{
		"VMName":        vmName,
		"SSHPublicKeys": sshPublicKeys,
	}

	return renderTemplate("user-data", templateUserData, templateData)
}

func (v *Virter) createCIData(sp libvirt.StoragePool, g ISOGenerator, vmConfig VMConfig) error {
	vmName := vmConfig.Name
	sshPublicKeys := vmConfig.SSHPublicKeys

	metaData, err := v.metaData(vmName)
	if err != nil {
		return err
	}

	userData, err := v.userData(vmName, sshPublicKeys)
	if err != nil {
		return err
	}

	files := map[string][]byte{
		"meta-data": []byte(metaData),
		"user-data": []byte(userData),
	}

	ciData, err := g.Generate(files)
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
