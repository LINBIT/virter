package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"

	sshclient "github.com/LINBIT/gosshclient"

	"github.com/LINBIT/virter/internal/virter"
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
	vmCmd.AddCommand(vmSSHCommand())
	vmCmd.AddCommand(vmCpCommand())
	return vmCmd
}

func containerProvider() string {
	return viper.GetString("container.provider")
}

// SSHClientBuilder builds SSH shell clients
type SSHClientBuilder struct {
}

// NewShellClient returns an SSH shell client
func (SSHClientBuilder) NewShellClient(hostPort string, sshConfig ssh.ClientConfig) virter.ShellClient {
	return sshclient.NewSSHClient(hostPort, sshConfig)
}

func extraAuthorizedKeys() ([]string, error) {
	publicKeys := []string{}

	userPublicKey := viper.GetString("auth.user_public_key")
	if userPublicKey != "" {
		publicKeys = append(publicKeys, userPublicKey)
	}

	return publicKeys, nil
}

func suggestVmNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	v, err := InitVirter()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer v.ForceDisconnect()

	vms, err := v.ListVM()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	filtered := make([]string, 0, len(vms))
outer:
	for _, vm := range vms {
		for _, arg := range args {
			if arg == vm {
				continue outer
			}
		}

		filtered = append(filtered, vm)
	}

	return filtered, cobra.ShellCompDirectiveNoFileComp
}
