package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
)

func initSSHKeys(publicPath string, privatePath string) error {
	publicExists, err := fileExists(publicPath)
	if err != nil {
		return err
	}

	privateExists, err := fileExists(privatePath)
	if err != nil {
		return err
	}

	if publicExists && privateExists {
		return nil
	} else if publicExists || privateExists {
		return fmt.Errorf("only one of '%s' and '%s' exist", publicPath, privatePath)
	}

	log.Printf("Initialization - Generate key pair '%s', '%s'", publicPath, privatePath)
	err = makeSSHKeyPair(publicPath, privatePath)
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	return nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if !os.IsNotExist(err) {
		return false, fmt.Errorf("error checking for existence of file '%s': %w", path, err)
	}

	return false, nil
}

func makeSSHKeyPair(publicPath string, privatePath string) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// generate and write private key as PEM
	privateKeyFile, err := os.OpenFile(privatePath, os.O_CREATE|os.O_WRONLY, 0600)
	defer privateKeyFile.Close()
	if err != nil {
		return err
	}
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(publicPath, ssh.MarshalAuthorizedKey(pub), 0644)
}
