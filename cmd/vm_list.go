package cmd

import (
	"strconv"

	"github.com/rodaine/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/LINBIT/virter/internal/virter"
)

func vmListCommand() *cobra.Command {
	existsCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all VMs",
		Long:    `List all VMs along with their ID and access network if they were created by Virter`,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			vms, err := v.VMList()
			if err != nil {
				log.Fatal(err)
			}

			vmInfos := make([]*virter.VMInfo, 0, len(vms))

			for _, vm := range vms {
				vmInfo, err := v.VMInfo(vm)
				if err != nil {
					log.Fatal(err)
				}

				vmInfos = append(vmInfos, vmInfo)
			}

			t := table.New("Name", "ID", "Access Network")
			for _, val := range vmInfos {
				id := ""
				if val.ID != 0 {
					id = strconv.Itoa(int(val.ID))
				}
				t.AddRow(val.Name, id, val.AccessNetwork)
			}
			t.Print()
		},
		ValidArgsFunction: suggestVmNames,
	}
	return existsCmd
}
