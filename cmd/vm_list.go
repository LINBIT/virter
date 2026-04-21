package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/rodaine/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/LINBIT/virter/internal/virter"
)

var validSortColumns = []string{"name", "id", "network", "state"}

func vmListCommand() *cobra.Command {
	sortBy := "name"

	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all VMs",
		Long:    `List all VMs along with their ID and access network if they were created by Virter`,
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if !slices.Contains(validSortColumns, sortBy) {
				log.Fatalf("invalid sort column %q, valid columns: %s", sortBy, strings.Join(validSortColumns, ", "))
			}

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

			slices.SortFunc(vmInfos, func(a, b *virter.VMInfo) int {
				var primary int
				switch sortBy {
				case "id":
					primary = int(a.ID) - int(b.ID)
				case "network":
					primary = strings.Compare(a.AccessNetwork, b.AccessNetwork)
				case "state":
					primary = boolToInt(a.Running) - boolToInt(b.Running)
				default:
					return strings.Compare(a.Name, b.Name)
				}
				return cmp.Or(primary, strings.Compare(a.Name, b.Name))
			})

			t := table.New("Name", "ID", "Access Network", "State")
			for _, val := range vmInfos {
				id := ""
				if val.ID != 0 {
					id = strconv.Itoa(int(val.ID))
				}
				state := "shut off"
				if val.Running {
					state = "running"
				}
				t.AddRow(val.Name, id, val.AccessNetwork, state)
			}
			t.Print()
		},
		ValidArgsFunction: suggestVmNames,
	}

	listCmd.Flags().StringVar(&sortBy, "sort", sortBy, fmt.Sprintf("sort by column (%s)", strings.Join(validSortColumns, ", ")))
	_ = listCmd.RegisterFlagCompletionFunc("sort", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return validSortColumns, cobra.ShellCompDirectiveNoFileComp
	})

	return listCmd
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
