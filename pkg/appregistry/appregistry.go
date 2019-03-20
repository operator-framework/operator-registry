package appregistry

import (
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/sqlite"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewLoader(kubeconfig string, logger *logrus.Entry, legacy bool) (*AppregistryLoader, error) {
	kubeClient, err := NewKubeClient(kubeconfig, logger)
	if err != nil {
		return nil, err
	}

	var specifier OperatorSourceSpecifier
	if legacy {
		logger.Info("operator source CR is being used.")
		p, err := NewOperatorSourceCRSpecifier(kubeconfig, logger)
		if err != nil {
			return nil, err
		}

		specifier = p
	} else {
		specifier = &registrySpecifier{}
	}

	return &AppregistryLoader{
		logger: logger,
		input: &inputParser{
			sourceSpecifier: specifier,
		},
		downloader: &downloader{
			logger:     logger,
			kubeClient: *kubeClient,
		},
		merger: &merger{
			logger: logger,
			parser: &manifestYAMLParser{},
		},
		loader: &dbLoader{
			logger: logger,
		},
	}, nil
}

type AppregistryLoader struct {
	logger     *logrus.Entry
	input      *inputParser
	downloader *downloader
	merger     *merger
	loader     *dbLoader
}

func (a *AppregistryLoader) Load(dbName string, csvSources []string, csvPackages string) (store *sqlite.SQLQuerier, err error) {
	a.logger.Infof("operator source(s) specified are - %s", csvSources)
	a.logger.Infof("package(s) specified are - %s", csvPackages)

	input, err := a.input.Parse(csvSources, csvPackages)
	if err != nil {
		a.logger.Errorf("the following error(s) occurred while parsing input - %v", err)

		if input == nil || !input.IsGoodToProceed() {
			a.logger.Info("can't proceed, bailing out")
			return nil, err
		}
	}

	a.logger.Info("input has been sanitized")
	a.logger.Infof("sources: %s", input.Sources)
	a.logger.Infof("packages: %s", input.Packages)

	rawManifests, err := a.downloader.Download(input)
	if err != nil {
		a.logger.Errorf("The following error occurred while downloading - %v", err)

		if len(rawManifests) == 0 {
			a.logger.Info("No package manifest downloaded")
			return nil, err
		}
	}

	a.logger.Infof("download complete - %d repositories have been downloaded", len(rawManifests))

	data, err := a.merger.Merge(rawManifests)
	if err != nil {
		a.logger.Errorf("The following error occurred while processing manifest - %v", err)

		if data == nil {
			a.logger.Info("No operator manifest bundled")
			return nil, err
		}
	}

	a.logger.Info("all manifest(s) have been merged into one")
	a.logger.Info("loading into sqlite database")

	store, err = a.loader.LoadToSQLite(dbName, data)
	return
}

func NewKubeClient(kubeconfig string, logger *logrus.Entry) (clientset *kubernetes.Clientset, err error) {
	var config *rest.Config

	if kubeconfig != "" {
		logger.Infof("Loading kube client config from path %q", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		logger.Infof("Using in-cluster kube client config")
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		err = fmt.Errorf("Cannot load config for REST client: %v", err)
		return
	}

	clientset, err = kubernetes.NewForConfig(config)
	return
}
