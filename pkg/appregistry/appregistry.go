package appregistry

import (
	"fmt"

	"github.com/operator-framework/operator-registry/pkg/registry"
	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewLoader returns a new instance of AppregistryLoader.
//
// kubeconfig specifies the location of kube configuration file.
// dbName specifies the database name to be used for sqlite.
// downloadPath specifies the folder where the downloaded nested bundle(s) will
// be stored.
func NewLoader(kubeconfig string, dbName string, downloadPath string, logger *logrus.Entry, legacy bool) (*AppregistryLoader, error) {
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

	decoder, err := NewManifestDecoder(logger, downloadPath)
	if err != nil {
		return nil, err
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
		decoder: decoder,
		loader:  NewDbLoader(dbName, logger),
	}, nil
}

type AppregistryLoader struct {
	logger     *logrus.Entry
	input      *inputParser
	downloader *downloader
	decoder    *manifestDecoder
	loader     *dbLoader
}

func (a *AppregistryLoader) Load(csvSources []string, csvPackages string) registry.Query {
	if err := a.load(csvSources, csvPackages); err != nil {
		a.loader.AddLoadError(newAppRegistryLoadError(err))
	}

	return a.loader.GetStore()
}

func (a *AppregistryLoader) load(csvSources []string, csvPackages string) error {
	a.logger.Infof("operator source(s) specified are - %s", csvSources)
	a.logger.Infof("package(s) specified are - %s", csvPackages)

	var errs []error
	input, err := a.input.Parse(csvSources, csvPackages)
	if err != nil {
		a.logger.Errorf("the following error(s) occurred while parsing input - %v", err)
		errs = append(errs, err)

		if input == nil || !input.IsGoodToProceed() {
			a.logger.Info("can't proceed, bailing out")
			return err
		}
	}

	a.logger.Info("input has been sanitized")
	a.logger.Infof("sources: %s", input.Sources)
	a.logger.Infof("packages: %s", input.Packages)

	rawManifests, err := a.downloader.Download(input)
	if err != nil {
		a.logger.Errorf("The following error occurred while downloading - %v", err)
		errs = append(errs, err)

		if len(rawManifests) == 0 {
			a.logger.Info("No package manifest downloaded")
			return utilerrors.NewAggregate(errs)
		}
	}

	a.logger.Infof("download complete - %d repositories have been downloaded", len(rawManifests))

	// The set of operator manifest(s) downloaded is a collection of both
	// flattened single file yaml and nested operator bundle(s).
	result, err := a.decoder.Decode(rawManifests)
	if err != nil {
		a.logger.Errorf("The following error occurred while decoding manifest - %v", err)
		errs = append(errs, err)

		if result.IsEmpty() {
			a.logger.Info("No operator manifest decoded")
			return utilerrors.NewAggregate(errs)
		}
	}

	a.logger.Infof("decoded %d flattened and %d nested operator manifest(s)", result.FlattenedCount, result.NestedCount)

	if result.Flattened != nil {
		a.logger.Info("loading flattened operator manifest(s) into sqlite")
		if err := a.loader.LoadFlattenedToSQLite(result.Flattened); err != nil {
			errs = append(errs, err)
		}
	}

	if result.NestedCount > 0 {
		a.logger.Infof("loading nested operator bundle(s) from %s into sqlite", result.NestedDirectory)
		if err := a.loader.LoadBundleDirectoryToSQLite(result.NestedDirectory); err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
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

func (a *AppregistryLoader) LoadErrors() []registry.LoadError {
	return a.loader.LoadErrors()
}

func (a *AppregistryLoader) LoadErrorsByType(errType registry.LoadErrorType) []registry.LoadError {
	return a.loader.LoadErrorsByType(errType)
}
