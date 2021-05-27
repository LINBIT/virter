package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"

	"github.com/LINBIT/virter/internal/virter"
)

func imageCommand() *cobra.Command {
	imageCmd := &cobra.Command{
		Use:   "image",
		Short: "Image related subcommands",
		Long:  `Image related subcommands.`,
	}

	imageCmd.AddCommand(imageBuildCommand())
	imageCmd.AddCommand(imagePullCommand())
	imageCmd.AddCommand(imageLsCommand())
	imageCmd.AddCommand(imageRmCommand())
	imageCmd.AddCommand(imageLoadCommand())
	imageCmd.AddCommand(imageSaveCommand())
	imageCmd.AddCommand(imagePushCommand())
	imageCmd.AddCommand(imagePruneCommand())

	return imageCmd
}

type mpbProgress struct {
	*mpb.Progress
}

func DefaultProgressFormat(p *mpb.Progress) virter.ProgressOpt {
	return &mpbProgress{Progress: p}
}

func (m *mpbProgress) NewBar(name, operation string, total int64) *mpb.Bar {
	if len(name) > 24 {
		name = name[:24]
	}

	return m.Progress.AddBar(
		total,
		mpb.PrependDecorators(
			decor.Name(name, decor.WC{W: len(name) + 1, C: decor.DidentRight}),
			decor.OnComplete(decor.Name(operation, decor.WCSyncWidthR), fmt.Sprintf("%s done", operation)),
		),
		mpb.AppendDecorators(decor.CountersKibiByte("%.2f / %.2f")),
	)
}
