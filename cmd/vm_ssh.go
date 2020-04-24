package cmd

import (
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func vmSSHCommand() *cobra.Command {
	sshCmd := &cobra.Command{
		Use:   "ssh vm_name",
		Short: "Run an interactive ssh shell in a VM",
		Long:  `Run an interactive ssh shell in a VM.`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}
			defer v.Disconnect()

			privateKey, err := loadPrivateKey()
			if err != nil {
				log.Fatal(err)
			}

			if err := v.VMSSHSession(context.TODO(), args[0], privateKey); err != nil {
				log.Fatal(err)
			}
		},
	}
	return sshCmd
}
