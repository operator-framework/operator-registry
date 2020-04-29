package version

import (
	"fmt"
	"runtime"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type opmVersionInfo struct {
	Commit    string
	FallBack  string 
	GoVersion string
	Version   string
}

const (
	FALLBACK_VERSION string = "v1.12.2"
)

var (
	version string
	commit string

	versionWrapper *opmVersionInfo = &opmVersionInfo{
		Commit: commit,
		FallBack : FALLBACK_VERSION,
		Version: version, 
		GoVersion: fmt.Sprintf("%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
	}
)

// AddVersionCommand adds the version command to the given parent command.
func NewVersionCommand() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Prints the version of opm",
		Run: func(cmd *cobra.Command, args []string) {
			c := versionWrapper.Commit
			v := versionWrapper.Version
			g := versionWrapper.GoVersion

			if v == "" {
				v = versionWrapper.FallBack
			}

			logger := logrus.WithFields(logrus.Fields{"Version": v, "commit": c, "GoVersion": g})

			logger.Info("opm version")
		},
	}

	return versionCmd
}
