package virter_test

import (
	"golang.org/x/crypto/ssh"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/internal/virter/mocks"
)

//go:generate mockery --name=ShellClient
//go:generate mockery --name=AfterNotifier
//go:generate mockery --name=NetworkCopier --dir=../../pkg/netcopy
//go:generate mockery --name=KeyStore --dir=../../pkg/sshkeys

const poolName = "some-pool"
const networkName = "some-network"
const imageName = "some-image"

type MockShellClientBuilder struct {
	ShellClient virter.ShellClient
}

func (b MockShellClientBuilder) NewShellClient(hostPort string, sshconfig ssh.ClientConfig) virter.ShellClient {
	return b.ShellClient
}

func newMockKeystore() *mocks.KeyStore {
	keystore := new(mocks.KeyStore)
	keystore.On("PublicKey").Return([]byte{})
	keystore.On("Auth").Return([]ssh.AuthMethod{})
	keystore.On("KeyBytes").Return([]byte{})
	keystore.On("KeyPath").Return("")
	return keystore
}
