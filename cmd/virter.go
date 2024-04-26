package cmd

import (
	"fmt"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/sshkeys"
)

// InitVirter initializes virter by connecting to the local libvirt instance and configures the ssh keystore.
func InitVirter() (*virter.Virter, error) {
	l := libvirt.NewWithDialer(dialers.NewLocal(
		dialers.WithSocket(viper.GetString("libvirt.socket")),
		dialers.WithLocalTimeout(2*time.Second),
	))
	if err := l.ConnectToURI(libvirt.ConnectURI(viper.GetString("libvirt.uri"))); err != nil {
		return nil, fmt.Errorf("failed to connect to libvirt socket: %w", err)
	}

	pool := viper.GetString("libvirt.pool")
	network := viper.GetString("libvirt.network")

	privateKeyPath := viper.GetString("auth.virter_private_key_path")
	publicKeyPath := viper.GetString("auth.virter_public_key_path")

	keyStore, err := sshkeys.NewKeyStore(privateKeyPath, publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load ssh key store: %w", err)
	}

	return virter.New(l, pool, network, keyStore), nil
}
