package cmd

import (
	"fmt"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
)

func rmMultiple(v *virter.Virter, vms []string) error {
	var errs error
	for _, vm := range vms {
		err := v.VMRm(vm)
		if err != nil {
			e := fmt.Errorf("failed to remove VM '%s': %w", vm, err)
			errs = multierror.Append(errs, e)
		}
	}
	return errs
}

func vmRmCommand() *cobra.Command {
	rmCmd := &cobra.Command{
		Use:   "rm vm_name [vm_name...]",
		Short: "Remove virtual machines",
		Long:  `Remove one or multiple virtual machines including all data.`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			err = rmMultiple(v, args)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	return rmCmd
}
