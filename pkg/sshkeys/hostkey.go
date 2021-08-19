package sshkeys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"strings"

	"golang.org/x/crypto/ssh"
)

type HostKey interface {
	PrivateKey() string
	PublicKey() string
}

func NewRSAHostKey() (HostKey, error) {
	privateRsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return &rsaHostKey{
		privateKey: privateRsaKey,
	}, nil
}

type rsaHostKey struct {
	privateKey *rsa.PrivateKey
}

func (s *rsaHostKey) PrivateKey() string {
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(s.privateKey)}
	return string(pem.EncodeToMemory(privateKeyPEM))
}

func (s *rsaHostKey) PublicKey() string {
	publicKey, err := ssh.NewSignerFromSigner(s.privateKey)
	if err != nil {
		panic(fmt.Sprintf("failed to convert generated rsa key: %v", err))
	}

	return string(ssh.MarshalAuthorizedKey(publicKey.PublicKey()))
}

// Create a new KnownHosts instance. Doesn't trust any hosts by default
func NewKnownHosts() KnownHosts {
	return &knownHosts{
		keymap: map[string][]string{},
	}
}

// A go implementation of ~/.ssh/known_hosts
type KnownHosts interface {
	// Add the key for use by the given hosts
	AddHost(key string, hosts ...string)
	// Returns a HostKeyCallback and HostKeyAlgorithms for use in ssh clients with the currently known hosts
	AsHostKeyConfig() (ssh.HostKeyCallback, []string)
	// Writes a "ssh" compatible representation of the known hosts to the given writer
	AsKnownHostsFile(writer io.Writer) error
}

type knownHosts struct {
	keymap map[string][]string
}

func (k *knownHosts) AddHost(key string, hosts ...string) {
	k.keymap[key] = hosts
}

func (k *knownHosts) AsHostKeyConfig() (ssh.HostKeyCallback, []string) {
	return func(dialName string, remote net.Addr, key ssh.PublicKey) error {
			entries, ok := k.keymap[string(ssh.MarshalAuthorizedKey(key))]
			if !ok {
				return fmt.Errorf("ssh: host key mismatch")
			}

			strippedDialName := strings.Split(dialName, ":")[0]
			strippedRemoteName := strings.Split(remote.String(), ":")[0]

			for _, entry := range entries {
				if entry == strippedDialName || entry == strippedRemoteName {
					return nil
				}
			}

			return fmt.Errorf("ssh: host key mismatch")
		}, []string{
			ssh.KeyAlgoRSA,
			ssh.SigAlgoRSASHA2256,
			ssh.SigAlgoRSASHA2512,
		}
}

func (k *knownHosts) AsKnownHostsFile(writer io.Writer) error {
	for key, entries := range k.keymap {
		allowedHosts := strings.Join(entries, ",")
		_, err := writer.Write([]byte(allowedHosts))
		if err != nil {
			return err
		}

		_, err = writer.Write([]byte(" "))
		if err != nil {
			return err
		}

		_, err = writer.Write([]byte(key))
		if err != nil {
			return err
		}
	}

	return nil
}
