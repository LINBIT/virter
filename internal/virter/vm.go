package virter

import (
	"bytes"
	"fmt"
)

// ISOGenerator generates ISO images from file data
type ISOGenerator interface {
	Generate(files map[string][]byte) ([]byte, error)
}

// VMRun starts a VM.
func (v *Virter) VMRun(g ISOGenerator, vmName string) error {
	metaData, err := v.metaData(vmName)
	if err != nil {
		return err
	}

	userData, err := v.userData(vmName)
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

	xml, err := v.volumeCIDataXML(ciDataVolName(vmName))
	if err != nil {
		return err
	}

	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	sv, err := v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create storage volume: %w", err)
	}

	err = v.libvirt.StorageVolUpload(sv, bytes.NewReader(ciData), 0, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to transfer data from URL to libvirt: %w", err)
	}

	return nil
}

func ciDataVolName(vmName string) string {
	return vmName + "-cidata"
}

func (v *Virter) metaData(vmName string) (string, error) {
	templateData := map[string]interface{}{
		"VMName": vmName,
	}

	return v.renderTemplate(templateMetaData, templateData)
}

func (v *Virter) userData(vmName string) (string, error) {
	templateData := map[string]interface{}{
		"VMName":       vmName,
		"SSHPublicKey": sshPublicKey,
	}

	return v.renderTemplate(templateUserData, templateData)
}

func (v *Virter) volumeCIDataXML(name string) (string, error) {
	templateData := map[string]interface{}{
		"VolumeName": name,
	}

	return v.renderTemplate(templateCIData, templateData)
}

const templateMetaData = "meta-data"
const templateUserData = "user-data"
const templateCIData = "volume-cidata.xml"
