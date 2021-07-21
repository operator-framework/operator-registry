package generate

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/platforms"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry/builder"
	"github.com/spf13/cobra"
	"strings"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate <path to declarative config directory>",
		Short: "Generate an oci-archive index image for a declarative config directory",
		Args:  cobra.MinimumNArgs(1),
		RunE: generate,
	}
	cmd.Flags().StringP("binary-image", "i", "quay.io/joelanford/opm:a8c80ac9", "container image for on-image `opm` command")
	cmd.Flags().StringP("tag", "t", "", "custom tag for container image being built")
	cmd.Flags().Bool("generate", false, "generate a dockerfile")
	cmd.Flags().StringP("out-dockerfile", "d", "", "if generating the dockerfile, this flag is used to (optionally) specify a dockerfile name")
	cmd.Flags().StringP("output", "o", "", "name of the output archive")
	cmd.Flags().StringSlice("labels", nil, "list of label key=value pairs to apply on the index image")

	return cmd
}

func generate(cmd *cobra.Command, args []string) error {
	tag, err := cmd.Flags().GetString("tag")
	if err != nil {
		return err
	}

	from, err := cmd.Flags().GetString("binary-image")
	if err != nil {
		return err
	}

	labels := map[string]string{}
	labelList, err := cmd.Flags().GetStringSlice("labels")
	if err != nil {
		return err
	}
	for _, l := range labelList {
		parts := strings.Split(l, "=")
		if len(parts) != 2 {
			return fmt.Errorf("invalid label format %s: expected KEY=VALUE", l)
		}
		labels[parts[0]] = parts[1]
	}

	generate, err := cmd.Flags().GetBool("generate")
	if err != nil {
		return err
	}

	dockerFilePrefix, err := cmd.Flags().GetString("out-dockerfile")
	if err != nil {
		return err
	}

	outputArchive, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}

	if !generate && len(outputArchive) == 0 {
		return fmt.Errorf("require at least one of --output or --out-dockerfile to be specified")
	}

	r, err := containerdregistry.NewRegistry()
	if err != nil {
		return err
	}

	ctx := context.TODO()
	b, err := builder.NewImageBuilder(ctx, tag, *r, builder.FromImage(image.SimpleReference(from)))
	if err != nil {
		return err
	}

	matcher := platforms.All
	if err := b.SetUser(ctx, matcher,"1001"); err != nil {
		return err
	}
	if err := b.ExposePorts(ctx, matcher, "50051"); err != nil {
		return err
	}
	if err := b.Add(ctx, matcher, args[0], "/database"); err != nil {
		return err
	}
	if err := b.SetEntrypoint(ctx, matcher, []string{"/bin/opm"}); err != nil {
		return err
	}
	if err := b.SetEntrypoint(ctx, matcher, []string{"serve", "/database"}); err != nil {
		return err
	}
	if err := b.SetLabels(ctx, matcher, labels); err != nil {
		return err
	}

	if generate {
		if err := b.GenerateDockerfile(ctx, dockerFilePrefix); err != nil {
			return err
		}
	}

	if len(outputArchive) != 0 {
		if err := b.ExportImageToOCIArchive(ctx, outputArchive); err != nil {
			return err
		}
	}
	return nil
}