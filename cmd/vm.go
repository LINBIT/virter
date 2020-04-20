package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/pkg/tcpping"
)

func vmCommand() *cobra.Command {
	vmCmd := &cobra.Command{
		Use:   "vm",
		Short: "Virtual machine related subcommands",
		Long:  `Virtual machine related subcommands.`,
	}

	vmCmd.AddCommand(vmCommitCommand())
	vmCmd.AddCommand(vmExecCommand())
	vmCmd.AddCommand(vmRmCommand())
	vmCmd.AddCommand(vmRunCommand())
	return vmCmd
}

func dockerConnect() (*client.Client, error) {
	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("could not connect to Docker %w", err)
	}

	return docker, nil
}

func dockerContext() (context.Context, context.CancelFunc) {
	dockerTimeout := viper.GetDuration("time.docker_timeout")
	return context.WithTimeout(context.Background(), dockerTimeout)
}

func newSSHPinger() tcpping.TCPPinger {
	return tcpping.TCPPinger{
		Count:  viper.GetInt("time.ssh_ping_count"),
		Period: viper.GetDuration("time.ssh_ping_period"),
	}

}

func loadPublicKeys() ([]string, error) {
	publicKeys := []string{}

	publicKeyPath := viper.GetString("auth.virter_public_key_path")
	publicKey, err := ioutil.ReadFile(publicKeyPath)
	if err != nil {
		return publicKeys, fmt.Errorf("failed to load public key from %s: %w", publicKeyPath, err)
	}

	publicKeys = append(publicKeys, strings.TrimSpace(string(publicKey)))

	userPublicKey := viper.GetString("auth.user_public_key")
	if userPublicKey != "" {
		publicKeys = append(publicKeys, userPublicKey)
	}

	return publicKeys, nil
}

func getPrivateKeyPath() string {
	return viper.GetString("auth.virter_private_key_path")
}

func loadPrivateKey() ([]byte, error) {
	privateKeyPath := getPrivateKeyPath()
	privateKey, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key from '%s': %w", privateKeyPath, err)
	}

	return privateKey, nil
}
