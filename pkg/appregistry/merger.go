package appregistry

import (
	"github.com/operator-framework/operator-registry/pkg/apprclient"
	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type merger struct {
	logger *logrus.Entry
	parser ManifestYAMLParser
}

// Merge merges a set of operator manifest(s) into one.
//
// For a given operator source we have N ( N >= 1 ) repositories within the
// given registry namespace. It is required for each repository to contain
// manifest for a single operator.
//
// Once downloaded we can use this function to merge manifest(s) from all
// relevant repositories into an uber manifest.
//
// We assume that all CRD(s), CSV(s) and package(s) are globally unique.
// Otherwise we will fail to load the uber manifest into sqlite.
func (m *merger) Merge(rawManifests []*apprclient.OperatorMetadata) (manifests *RawOperatorManifestData, err error) {
	allErrors := []error{}
	merged := StructuredOperatorManifestData{}

	for _, rawManifest := range rawManifests {
		manifest, err := m.parser.Unmarshal(rawManifest.RawYAML)
		if err != nil {
			allErrors = append(allErrors, err)
			m.logger.Infof("skipping repository due to parsing error - %s", rawManifest.RegistryMetadata)

			continue
		}

		merged.Packages = append(merged.Packages, manifest.Packages...)
		merged.CustomResourceDefinitions = append(merged.CustomResourceDefinitions, manifest.CustomResourceDefinitions...)
		merged.ClusterServiceVersions = append(merged.ClusterServiceVersions, manifest.ClusterServiceVersions...)
	}

	manifests, err = m.parser.Marshal(&merged)
	if err != nil {
		allErrors = append(allErrors, err)
	}

	err = utilerrors.NewAggregate(allErrors)
	return
}
