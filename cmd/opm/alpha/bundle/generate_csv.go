package bundle

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/pkg/lib/synthesize"
)

// newCsvGenerateCmd returns a command that will generate csv for the bundle.
func newCsvGenerateCmd() *cobra.Command {
	generateCsvCmd := &cobra.Command{
		Use:   "generate-csv",
		Short: "Generates ClusterServiceVersion file based on some Kube manifests with some extra metadata",
		Long: `Generates ClusterServiceVersion file based on some Kube manifests with some extra information
		present in the metadata directory of the bundle including annotations.yaml pointing to the locations of
		manifest and metadata directory, olm.yaml that includes non-inferable information about the CSV, and 
		dependencies.yaml listing out the required apis and packages for the bundle if any.

		This command requires bundle to be in a format that contains metadata and manifests folders.

        $ opm alpha bundle generate-csv --directory /test/0.1.0

		Note:
		* All manifests yaml must be in the same directory.
		* All files in metadata directory do not have strict naming convention.
        `,
		RunE: generateCsvFunc,
	}

	generateCsvCmd.Flags().StringVarP(&dirBuildArgs, "directory", "d", "",
		"The location of bundle directory with a format that contains metadata and manifests folders "+
			"with non-inferable csv information and annotations.yaml file in the metadata folder.")
	if err := generateCsvCmd.MarkFlagRequired("directory"); err != nil {
		log.Fatalf("Failed to mark `directory` flag for `generate-csv` subcommand as required")
	}

	return generateCsvCmd
}

func generateCsvFunc(cmd *cobra.Command, args []string) error {
	err := synthesize.GenerateCSV(dirBuildArgs)
	if err != nil {
		return err
	}

	return nil
}
