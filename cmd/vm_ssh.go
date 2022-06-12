package cmd

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func vmSSHCommand() *cobra.Command {
	var user string

	sshCmd := &cobra.Command{
		Use:   "ssh vm_name",
		Short: "Run an interactive ssh shell in a VM",
		Long:  `Run an interactive ssh shell in a VM.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			if err := v.VMSSHSession(context.TODO(), args[0], user); err != nil {
				log.Fatal(err)
			}
		},
		ValidArgsFunction: suggestVmNames,
	}
	sshCmd.Flags().StringVarP(&user, "user", "u", "root", "Remote user for ssh session")

	return sshCmd
}
