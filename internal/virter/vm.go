package virter

import (
	"bytes"
	"fmt"

	"github.com/digitalocean/go-libvirt"
)

// ISOGenerator generates ISO images from file data
type ISOGenerator interface {
	Generate(files map[string][]byte) ([]byte, error)
}

// VMRun starts a VM.
func (v *Virter) VMRun(g ISOGenerator, imageName string, vmName string) error {
	sp, err := v.libvirt.StoragePoolLookupByName(v.storagePoolName)
	if err != nil {
		return fmt.Errorf("could not get storage pool: %w", err)
	}

	err = v.createCIData(sp, g, vmName)
	if err != nil {
		return err
	}

	err = v.createVMVolume(sp, imageName, vmName)
	if err != nil {
		return err
	}

	return nil
}

func (v *Virter) createCIData(sp libvirt.StoragePool, g ISOGenerator, vmName string) error {
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

	xml, err := v.ciDataVolumeXML(ciDataVolName(vmName))
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

func (v *Virter) ciDataVolumeXML(name string) (string, error) {
	templateData := map[string]interface{}{
		"VolumeName": name,
	}

	return v.renderTemplate(templateCIData, templateData)
}

func (v *Virter) createVMVolume(sp libvirt.StoragePool, imageName string, vmName string) error {
	backingVol, err := v.libvirt.StorageVolLookupByName(sp, imageName)
	if err != nil {
		return fmt.Errorf("could not get backing image volume: %w", err)
	}

	backingPath, err := v.libvirt.StorageVolGetPath(backingVol)
	if err != nil {
		return fmt.Errorf("could not get backing image path: %w", err)
	}

	xml, err := v.vmVolumeXML(vmName, backingPath)
	if err != nil {
		return err
	}

	_, err = v.libvirt.StorageVolCreateXML(sp, xml, 0)
	if err != nil {
		return fmt.Errorf("could not create VM boot volume: %w", err)
	}

	return nil
}

func (v *Virter) vmVolumeXML(name string, backingPath string) (string, error) {
	templateData := map[string]interface{}{
		"VolumeName":  name,
		"BackingPath": backingPath,
	}

	return v.renderTemplate(templateVMVolume, templateData)
}

const templateMetaData = "meta-data"
const templateUserData = "user-data"
const templateCIData = "volume-cidata.xml"
const templateVMVolume = "volume-vm.xml"
