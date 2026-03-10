package virter_test

import (
	"golang.org/x/crypto/ssh"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
)


const poolName = "some-pool"
const networkName = "some-network"
const imageName = "some-image"

type MockShellClientBuilder struct {
	ShellClient virter.ShellClient
}

func (b MockShellClientBuilder) NewShellClient(hostPort string, sshconfig ssh.ClientConfig) virter.ShellClient {
	return b.ShellClient
}

func newMockKeystore() *mocks.MockKeyStore {
	keystore := new(mocks.MockKeyStore)
	keystore.On("PublicKey").Return([]byte{})
	keystore.On("Auth").Return([]ssh.AuthMethod{})
	keystore.On("KeyBytes").Return([]byte{})
	keystore.On("KeyPath").Return("")
	return keystore
}
