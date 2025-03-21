package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func vmWaitReadyCommand() *cobra.Command {
	existsCmd := &cobra.Command{
		Use:   "wait-ready [vm_name...]",
		Short: "Wait for VMs to be ready for use",
		Long:  "Wait for VMs to be ready for use. Blocks until all VMs specified on the command line are reachable via SSH.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			var g errgroup.Group
			for i := range args {
				g.Go(func() error {
					return v.WaitVmReady(cmd.Context(), SSHClientBuilder{}, args[i], getReadyConfig())
				})
			}

			if err := g.Wait(); err != nil {
				log.Fatal(err)
			}
		},
		ValidArgsFunction: suggestVmNames,
	}
	return existsCmd
}
