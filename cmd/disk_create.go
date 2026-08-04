package cmd

import (
	"github.com/rck/unit"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func diskCreateCommand() *cobra.Command {
	var pool string
	var size *unit.Value

	createCmd := &cobra.Command{
		Use:   "create name",
		Short: "Create a shared disk",
		Long: `Create a raw disk that can be attached to multiple VMs at the same time
using "virter vm run --shared-disk". This can be used to simulate shared (SAN)
storage. Note that the guests need to use a cluster aware filesystem or lock
manager (for example lvmlockd with sanlock) to safely access the disk from
multiple VMs.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sizeKiB := uint64(size.Value / unit.DefaultUnits["K"])

			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			err = v.SharedDiskCreate(args[0], pool, sizeKiB)
			if err != nil {
				log.Fatalf("Error creating shared disk: %v", err)
			}
		},
		ValidArgsFunction: suggestNone,
	}

	u := unit.MustNewUnit(sizeUnits)
	size = u.MustNewValue(0, unit.None)
	createCmd.Flags().VarP(size, "size", "s", "Size of the shared disk")
	_ = createCmd.MarkFlagRequired("size")
	createCmd.Flags().StringVar(&pool, "pool", "", "Name of the storage pool to create the disk in (defaults to the configured storage pool)")

	return createCmd
}
