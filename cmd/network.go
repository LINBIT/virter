package cmd

import (
	"github.com/spf13/cobra"
)

func networkCommand() *cobra.Command {
	networkCmd := &cobra.Command{
		Use:   "network",
		Short: "Network related subcommands",
		Long:  `Network related subcommands.`,
	}

	networkCmd.AddCommand(networkHostCommand())
	networkCmd.AddCommand(networkLsCommand())
	networkCmd.AddCommand(networkAddCommand())
	networkCmd.AddCommand(networkRmCommand())
	networkCmd.AddCommand(networkListAttachedCommand())
	return networkCmd
}

func suggestNetworkNames(cmd *cobra.Command, args []string, tocomplete string) ([]string, cobra.ShellCompDirective) {
	v, err := InitVirter()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer v.ForceDisconnect()

	nets, err := v.NetworkList()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	filtered := make([]string, 0, len(nets))
outer:
	for _, net := range nets {
		for _, arg := range args {
			if arg == net.Name {
				continue outer
			}
		}

		filtered = append(filtered, net.Name)
	}

	return filtered, cobra.ShellCompDirectiveNoFileComp
}
