package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/rodaine/table"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func parseLibvirtTimestamp(raw string) (time.Time, error) {
	parts := strings.SplitN(raw, ".", 2)

	secsStr := parts[0]
	nsecsStr := "0"
	if len(parts) == 2 {
		nsecsStr = parts[1]
	}

	secs, err := strconv.Atoi(secsStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to convert string to timestamp: %w", err)
	}

	nsecs, err := strconv.Atoi(nsecsStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to convert string to timestamp: %w", err)
	}

	return time.Unix(int64(secs), int64(nsecs)), nil
}

func imageLsCommand() *cobra.Command {
	var listHttp bool

	lsCmd := &cobra.Command{
		Use:   "ls",
		Short: "List images",
		Long:  `List images that can be used to start VMs.`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			v, err := InitVirter()
			if err != nil {
				log.Fatal(err)
			}
			defer v.ForceDisconnect()

			if listHttp {
				reg := loadRegistry()
				entries, err := reg.List()
				if err != nil {
					log.WithError(err).Fatal("Error listing images")
				}
				t := table.New("Name", "URL")
				for name, entry := range entries {
					t.AddRow(name, entry.URL)
				}
				t.Print()
			} else {
				images, err := v.ImageList()
				if err != nil {
					log.WithError(err).Fatalf("Error listing images")
				}

				now := time.Now()

				t := table.New("Name", "Top Layer", "Created")
				for _, img := range images {
					top := img.TopLayer()

					diffId, err := top.DiffID()
					if err != nil {
						log.WithError(err).Warnf("skipping image %s", img.Name())
						continue
					}

					desc, err := top.Descriptor()
					if err != nil {
						log.WithError(err).Warnf("skipping image %s", img.Name())
						continue
					}

					lastModified, err := parseLibvirtTimestamp(desc.Target.Timestamps.Mtime)
					if err != nil {
						log.WithError(err).Warnf("skipping image %s", img.Name())
						continue
					}

					lastModifiedStr := units.HumanDuration(now.Sub(lastModified))

					t.AddRow(img.Name(), diffId, fmt.Sprintf("%s ago", lastModifiedStr))
				}
				t.Print()
			}
		},
	}

	lsCmd.Flags().BoolVar(&listHttp, "available", false, "List all images available from http registries")

	return lsCmd
}
