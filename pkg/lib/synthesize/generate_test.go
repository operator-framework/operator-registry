package synthesize

import (
	"io"
	"io/ioutil"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/operator-framework/operator-registry/pkg/lib/bundle"
)

func TestGenerateCSV(t *testing.T) {
	csvlessBundlePath := "./testdata/csvless_bundle/prometheus_v0.22.2"
	csvPrometheusBundleFilePath := "./testdata/csvless_bundle/prometheus_v0.22.2/manifests/olm.clusterserviceversion.yaml"
	annotationsPrometheusBundleFilePath := "./testdata/csvless_bundle/prometheus_v0.22.2/metadata/annotations.yaml"
	expectedCsvPrometheusBundleFilePath := "./testdata/csvless_bundle/prometheus_v0.22.2.clusterserviceversion.yaml"

	utilrand.Seed(100)

	// Test generating a CSV and annotations mediatype updates.
	assertMediaType(t, annotationsPrometheusBundleFilePath, bundle.PlainType)
	err := GenerateCSV(csvlessBundlePath)
	require.NoError(t, err)
	assertMediaType(t, annotationsPrometheusBundleFilePath, bundle.RegistryV1Type)
	assert.Equal(t, loadCSV(t, expectedCsvPrometheusBundleFilePath), loadCSV(t, csvPrometheusBundleFilePath))

	// Statically validate bundle with generated CSV.
	logger := log.NewEntry(log.New())
	bundleValidator := bundle.NewBundleValidator(logger)
	err = bundleValidator.ValidateBundleFormat(csvlessBundlePath)
	assert.NoErrorf(t, err, "error validating bundle format")
	err = bundleValidator.ValidateBundleContent(csvlessBundlePath + "/manifests")
	assert.NoErrorf(t, err, "error validating bundle content")

	// reset test directory
	copyFile(t, "./testdata/csvless_bundle/annotations.yaml", annotationsPrometheusBundleFilePath)

	// Test generating a CSV while deleting the previous one.
	PrometheusBundleWithCsvPath := "./testdata/csvless_bundle/prometheus_v0.22.2.withcsv"
	csvPrometheusBundleWithCsvFilePath := "./testdata/csvless_bundle/prometheus_v0.22.2.withcsv/manifests/olm." +
		"clusterserviceversion.yaml"
	expectedSmallCsvPrometheusBundleFilePath := "./testdata/csvless_bundle/prometheus_v0.22.2.small." +
		"clusterserviceversion.yaml"
	previousCSV := "./testdata/csvless_bundle/prometheus_v0.22.2.withcsv/" +
		"manifests/prometheus_v0.22.2.small.clusterserviceversion.yaml"

	err = GenerateCSV(PrometheusBundleWithCsvPath)
	require.NoError(t, err)
	assert.Equal(t, loadCSV(t, expectedSmallCsvPrometheusBundleFilePath), loadCSV(t, csvPrometheusBundleWithCsvFilePath))
	_, err = ioutil.ReadFile(previousCSV)
	assert.Errorf(t, err, "The previous CSV is not removed.")

	copyFile(t, expectedSmallCsvPrometheusBundleFilePath, previousCSV)
	os.Remove(csvPrometheusBundleWithCsvFilePath)
}

func loadCSV(t *testing.T, csvFilePath string) v1alpha1.ClusterServiceVersion {
	reader, err := os.Open(csvFilePath)
	require.NoError(t, err)

	csv := v1alpha1.ClusterServiceVersion{}
	decoder := utilyaml.NewYAMLOrJSONDecoder(reader, 30)
	require.NoError(t, decoder.Decode(&csv))

	return csv
}

func copyFile(t *testing.T, src, dst string) {
	in, err := os.Open(src)
	require.NoError(t, err)
	defer in.Close()

	out, err := os.Create(dst)
	require.NoError(t, err)

	defer out.Close()

	_, err = io.Copy(out, in)
	require.NoError(t, err)
}

func assertMediaType(t *testing.T, annotationsFile, mediaType string) {
	content, err := ioutil.ReadFile(annotationsFile)
	require.NoError(t, err)
	annotations, err := bundle.GetAnnotations(content)
	require.NoError(t, err)
	assert.Equal(t, annotations.GetMediatype(), mediaType)
}
