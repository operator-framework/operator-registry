package generate

import (
	"context"
	"os"

	"github.com/operator-framework/operator-registry/pkg/action"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const defaultDockerfileName = "index.Dockerfile"

type generate struct {
	configDir   string
	sourceImage string
	debug       bool
	logger      *logrus.Entry
}

func NewCmd() *cobra.Command {
	logger := logrus.New()
	g := generate{
		logger: logrus.NewEntry(logger),
	}
	cmd := &cobra.Command{
		Use:   "generate <configs_dir>",
		Short: "generate DockerFile",
		Long:  `generate DockerFile to be used to build index image with configs`,
		Args:  cobra.ExactArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			g.configDir = args[0]
			if g.debug {
				logger.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return g.run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&g.debug, "debug", false, "enable debug logging")
	cmd.Flags().StringVarP(&g.sourceImage, "source-image", "s", "quay.io/operator-framework/upstream-opm-builder", "the base/source image to build the index image on")
	return cmd
}

func (s *generate) run(ctx context.Context) error {

	f, err := os.Create(defaultDockerfileName)
	if err != nil {
		return err
	}

	request := action.GenerateDockerfileRequest{
		SourceImage: s.sourceImage,
		ConfigsDir:  s.configDir,
		File:        f,
	}
	generator := action.NewGenerator(s.logger)

	return generator.GenerateDockerfile(request)
}
