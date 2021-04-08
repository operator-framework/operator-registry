package unpack

import (
	"context"

	"github.com/operator-framework/operator-registry/pkg/action"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type unpack struct {
	image   string
	dir     string
	caFile  string
	skipTLS bool
	debug   bool
	logger  *logrus.Entry
}

func NewCmd() *cobra.Command {
	logger := logrus.New()
	u := unpack{
		logger: logrus.NewEntry(logger),
	}
	cmd := &cobra.Command{
		Use:   "unpack <image>",
		Short: "unpack an index image",
		Long:  "unpack an index image's content(i.e operators shipped with the index)",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			u.image = args[0]
			if u.debug {
				logger.SetLevel(logrus.DebugLevel)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return u.run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&u.debug, "debug", false, "enable debug logging")
	cmd.Flags().StringVarP(&u.caFile, "ca-file", "", "", "the root Certificates to use with this command")
	cmd.Flags().BoolVar(&u.skipTLS, "skip-tls", false, "disable TLS verification")
	cmd.Flags().StringVarP(&u.dir, "unpack-dir", "", "", "the directory the configs will be copied to")
	return cmd
}

func (u *unpack) run(ctx context.Context) error {

	reg, err := containerdregistry.NewRegistry()
	if err != nil {
		return err
	}
	defer func() {
		if err := reg.Destroy(); err != nil {
			u.logger.WithError(err).Warn("error destroying local cache")
		}
	}()

	unpacker := action.NewIndexUnpacker(u.logger, reg, ctx)
	request := action.UnpackRequest{
		Image:     u.image,
		UnpackDir: u.dir,
	}
	return unpacker.Unpack(request)
}
