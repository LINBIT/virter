package cmd

import (
	"context"

	"github.com/LINBIT/virter/pkg/netcopy"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func vmCpCommand() *cobra.Command {
	sshCmd := &cobra.Command{
		Use:   "cp [HOST:]SRC... [HOST:]DEST",
		Short: "Copy files and directories from and to VM",
		Long:  `Copy files and directories from and to VM`,
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			sourceSpec := args[:len(args)-1]
			destSpec := args[len(args)-1]

			copier := netcopy.NewRsyncNetworkCopier()

			if err := v.VMExecCopy(context.TODO(), copier, sourceSpec, destSpec); err != nil {
				log.Fatal(err)
			}
		},
	}
	return sshCmd
}
