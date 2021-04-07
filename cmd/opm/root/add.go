package root

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/util/retry"

	"github.com/operator-framework/operator-registry/pkg/action"
	"github.com/operator-framework/operator-registry/pkg/image"
	"github.com/operator-framework/operator-registry/pkg/image/containerdregistry"
	"github.com/operator-framework/operator-registry/pkg/lib/certs"
)

const (
	retryInterval = time.Second * 5
	timeout       = time.Minute * 1
)

type add struct {
	logger     *logrus.Entry
	configsDir string
	debug      bool
	caFile     string
	pullTool   string
	skipTLS    bool
}

func newAddCmd() *cobra.Command {
	logger := logrus.New()
	a := add{
		logger:   logrus.NewEntry(logger),
		pullTool: "none",
	}
	rootCmd := &cobra.Command{
		Use:   "add <configs_path> <bundle_image1> <bundle_image2>........<bundle_imageN>",
		Short: "add operator bundle/s to a catalog of packages",
		Long:  `add operator bundles to a directory of configs representing packages in the catalog`,
		Args:  cobra.MinimumNArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			a.configsDir = args[0]
			if a.debug {
				logger.SetLevel(logrus.DebugLevel)
			}
			return nil
		},

		RunE: a.addFunc,
	}

	rootCmd.Flags().BoolVar(&a.debug, "debug", false, "enable debug logging")
	rootCmd.Flags().StringVarP(&a.caFile, "ca-file", "", "", "the root Certificates to use with this command")
	rootCmd.Flags().BoolVar(&a.skipTLS, "skip-tls", false, "disable TLS verification")
	return rootCmd
}

func (a *add) addFunc(cmd *cobra.Command, args []string) error {
	rootCAs, err := certs.RootCAs(a.caFile)
	if err != nil {
		return fmt.Errorf("failed to get RootCAs: %v", err)
	}
	reg, err := containerdregistry.NewRegistry(containerdregistry.SkipTLS(a.skipTLS), containerdregistry.WithLog(a.logger), containerdregistry.WithRootCAs(rootCAs))
	if err != nil {
		return err
	}
	defer func() {
		if err := reg.Destroy(); err != nil {
			a.logger.Errorf("error destroying local cache: %v", err)
		}
	}()
	bundles := []action.InputBundle{}
	for _, ref := range args[1:] {
		simpleRef := image.SimpleReference(ref)
		tmpDir, err := ioutil.TempDir("./", "bundle_tmp")
		if err != nil {
			return fmt.Errorf("error creating temp directory to unpack bundle image %q in:%v", simpleRef.String(), err)
		}
		defer func() {
			a.logger.Infof("Removing temp directory %q bundle was unpacked in", tmpDir)
			if err := os.RemoveAll(tmpDir); err != nil {
				a.logger.Errorf("error removing temp directory %q bundle was unpacked in: %v", tmpDir, err)
			}
		}()
		nonRetryableRegex := regexp.MustCompile(`(error resolving name)`)
		a.logger.Infof("Pulling bundle %q", simpleRef.String())
		if err := retry.OnError(retry.DefaultRetry,
			func(err error) bool {
				if nonRetryableRegex.MatchString(err.Error()) {
					return false
				}
				a.logger.Warnf("  Error pulling image: %v. Retrying.", err)
				return true
			},
			func() error { return reg.Pull(cmd.Context(), simpleRef) }); err != nil {
			return fmt.Errorf("error pulling image %q into registry:%v", simpleRef.String(), err)
		}
		a.logger.Infof("Unpacking bundle %q into %q", simpleRef.String(), tmpDir)
		err = reg.Unpack(cmd.Context(), simpleRef, tmpDir)
		if err != nil {
			return fmt.Errorf("error unpacking image %q: %v", simpleRef.String(), err)
		}
		bundles = append(bundles, action.InputBundle{Dir: tmpDir, ImgRef: simpleRef})
	}
	request := action.AddConfigRequest{
		Bundles:    bundles,
		ConfigsDir: a.configsDir,
	}
	adder := action.NewBundleAdder(a.logger)
	return adder.AddToConfig(request)
}
