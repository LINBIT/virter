package virter_test

import (
	"golang.org/x/crypto/ssh"

	"github.com/LINBIT/virter/internal/virter"
)

//go:generate mockery -name=ShellClient
//go:generate mockery -name=AfterNotifier
//go:generate mockery -name=NetworkCopier

const poolName = "some-pool"
const networkName = "some-network"
const imageName = "some-image"

type MockShellClientBuilder struct {
	ShellClient virter.ShellClient
}

func (b MockShellClientBuilder) NewShellClient(hostPort string, sshconfig ssh.ClientConfig) virter.ShellClient {
	return b.ShellClient
}
