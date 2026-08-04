package cmd

import (
	"fmt"
	"slices"

	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func diskRmCommand() *cobra.Command {
	var pool string

	rmCmd := &cobra.Command{
		Use:   "rm name [name...]",
		Short: "Remove shared disks",
		Long:  `Remove one or more shared disks. A shared disk can only be removed after all VMs using it have been removed.`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			var errs error
			for _, name := range args {
				err := v.SharedDiskRm(name, pool)
				if err != nil {
					e := fmt.Errorf("failed to remove shared disk '%s': %w", name, err)
					errs = multierror.Append(errs, e)
				}
			}
			if errs != nil {
				log.Fatal(errs)
			}
		},
		ValidArgsFunction: suggestSharedDiskNames,
	}

	rmCmd.Flags().StringVar(&pool, "pool", "", "Name of the storage pool containing the disk (defaults to the configured storage pool)")

	return rmCmd
}

func suggestSharedDiskNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	v, err := InitVirter()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer v.ForceDisconnect()

	disks, err := v.SharedDiskList("")
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var names []string
	for _, disk := range disks {
		if slices.Contains(args, disk.Name) {
			// already mentioned in previous argument
			continue
		}

		names = append(names, disk.Name)
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}
