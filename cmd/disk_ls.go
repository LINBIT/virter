package cmd

import (
	"strings"

	"github.com/docker/go-units"
	"github.com/rodaine/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func diskLsCommand() *cobra.Command {
	var pool string

	lsCmd := &cobra.Command{
		Use:   "ls",
		Short: "List shared disks",
		Long:  `List shared disks and the VMs they are attached to.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			disks, err := v.SharedDiskList(pool)
			if err != nil {
				log.WithError(err).Fatal("Error listing shared disks")
			}

			t := table.New("Name", "Size", "Attached To")
			for _, disk := range disks {
				t.AddRow(disk.Name, units.BytesSize(float64(disk.SizeB)), strings.Join(disk.AttachedTo, ","))
			}
			t.Print()
		},
		ValidArgsFunction: suggestNone,
	}

	lsCmd.Flags().StringVar(&pool, "pool", "", "Name of the storage pool to list disks from (defaults to the configured storage pool)")

	return lsCmd
}
