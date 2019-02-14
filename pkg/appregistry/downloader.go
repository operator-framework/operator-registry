package appregistry

import (
	"fmt"

	"github.com/operator-framework/operator-marketplace/pkg/apis/marketplace/v1alpha1"
	marketplace "github.com/operator-framework/operator-marketplace/pkg/client/clientset/versioned"
	"github.com/operator-framework/operator-registry/pkg/apprclient"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// downloadItem encapsulates the data that is needed to download a specific repository.
type downloadItem struct {
	// Repository points to the repository and the particular release that needs
	// to be downloaded.
	RepositoryMetadata *apprclient.RegistryMetadata

	// Spec refers to the remote appregistry URL and remote registry namespace.
	Spec *v1alpha1.OperatorSourceSpec
}

func (d *downloadItem) String() string {
	return fmt.Sprintf("%s", d.RepositoryMetadata)
}

type downloader struct {
	logger *logrus.Entry
	client marketplace.Interface
}

// Download downloads manifest(s) associated with the specified package(s) from
// the corresponding operator source(s).
//
// We take a best effort approach in downloading.
//
// If an OperatorSource is not found, we skip it and move on to the next source.
func (d *downloader) Download(input *Input) (manifests []*apprclient.OperatorMetadata, err error) {
	items, err := d.Prepare(input)
	if err != nil {
		d.logger.Errorf("the following error(s) occurred while preparing the download list: %v", err)

		if len(items) == 0 {
			d.logger.Infof("download list is empty, bailing out: %s", input.Packages)
			return
		}
	}

	d.logger.Infof("resolved the following packages: %s", items)

	manifests, err = d.DownloadRepositories(items)

	return
}

// Prepare prepares the list of repositories to download by resolving each
// package specified to its corresponding operator source.
//
// If a package is specified more than once, the operator source that it
// resolves to the first time is picked.
//
// We apply a best-effort approach here, if a package is can't be resolved we
// log it and move on.
func (d *downloader) Prepare(input *Input) (items []*downloadItem, err error) {
	packageMap := input.PackagesToMap()
	itemMap := map[string]*downloadItem{}
	allErrors := []error{}

	for _, source := range input.Sources {
		if len(packageMap) == 0 {
			// All specified package(s) have already been resolved.
			break
		}

		spec, repositoryList, err := d.QuerySource(source)
		if err != nil {
			allErrors = append(allErrors, err)
			d.logger.Infof("skipping operator source due to error: %s", source)

			continue
		}

		for _, metadata := range repositoryList {
			// Repository name has a one to one mapping to operator/package name.
			// We use this as the key.
			key := metadata.Name

			if _, ok := packageMap[key]; ok {
				// The package specified has been resolved to this repository
				// name in remote registry.
				itemMap[key] = &downloadItem{
					RepositoryMetadata: metadata,
					Spec:               spec,
				}

				// Remove the package specified since it has been resolved.
				delete(packageMap, key)
			}
		}
	}

	// We might still have packages specified that have not been resolved.
	if len(packageMap) > 0 {
		d.logger.Infof("the following packages could not be resolved: %v", packageMap)
	}

	items = make([]*downloadItem, 0)
	for _, v := range itemMap {
		items = append(items, v)
	}

	err = utilerrors.NewAggregate(allErrors)
	return
}

// DownloadRepositories iterates through each download item and downloads
// operator manifest from the corresponding repository.
func (d *downloader) DownloadRepositories(items []*downloadItem) (manifests []*apprclient.OperatorMetadata, err error) {
	allErrors := []error{}

	manifests = make([]*apprclient.OperatorMetadata, 0)
	for _, item := range items {
		endpoint := item.Spec.Endpoint

		d.logger.Infof("downloading repository: %s from %s", item.RepositoryMetadata, endpoint)

		factory := apprclient.NewClientFactory()

		client, err := factory.New("appregistry", endpoint)
		if err != nil {
			allErrors = append(allErrors, err)
			d.logger.Infof("skipping repository: %s", item.RepositoryMetadata)

			continue
		}

		manifest, err := client.RetrieveOne(item.RepositoryMetadata.ID(), item.RepositoryMetadata.Release)
		if err != nil {
			allErrors = append(allErrors, err)
			d.logger.Infof("skipping repository: %s", item.RepositoryMetadata)

			continue
		}

		manifests = append(manifests, manifest)
	}

	err = utilerrors.NewAggregate(allErrors)
	return
}

// QuerySource retrives the OperatorSource object specified by key. It queries
// the registry namespace to list all the repositories associated with this
// operator source.
//
// The function returns the spec ( associated with the OperatorSource object )
// in the cluster and the list of repositories in remote registry associated
// with it.
func (d *downloader) QuerySource(key *types.NamespacedName) (spec *v1alpha1.OperatorSourceSpec, repositories []*apprclient.RegistryMetadata, err error) {
	opsrc, err := d.client.MarketplaceV1alpha1().OperatorSources(key.Namespace).Get(key.Name, metav1.GetOptions{})
	if err != nil {
		return
	}

	factory := apprclient.NewClientFactory()
	client, err := factory.New("appregistry", opsrc.Spec.Endpoint)
	if err != nil {
		return
	}

	repositories, err = client.ListPackages(opsrc.Spec.RegistryNamespace)
	if err != nil {
		return
	}

	spec = &opsrc.Spec
	return
}
