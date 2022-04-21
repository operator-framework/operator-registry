package veneer

import (
	"io/ioutil"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func nullLogger() *logrus.Entry {
	logger := logrus.New()
	logger.SetOutput(ioutil.Discard)
	return logrus.NewEntry(logger)
}

func NewCmd() *cobra.Command {
	runCmd := &cobra.Command{
		Use:   "render-veneer",
		Short: "Render a veneer type",
		Args:  cobra.NoArgs,
	}

	runCmd.AddCommand(newBasicVeneerRenderCmd())

	return runCmd
}
