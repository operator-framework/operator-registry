package validate

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/pkg/lib/config"
)

func NewCmd() *cobra.Command {
	logger := logrus.New()
	validate := &cobra.Command{
		Use:   "validate <directory>",
		Short: "Validate the declarative index config",
		Long:  "Validate the declarative config JSON file(s) in a given directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			directory := args[0]
			if _, err := os.Stat(directory); os.IsNotExist(err) {
				return err
			}

			if err := config.ValidateConfig(directory); err != nil {
				logger.Fatal(err)
			}
			return nil
		},
	}

	return validate
}
