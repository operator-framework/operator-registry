package appregistry

import (
	"archive/tar"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessWithFlattenedFormat(t *testing.T) {
	checker := formatChecker{}

	// Simulate a flattened format as used by Operator Courier.
	root := &tar.Header{
		Typeflag: tar.TypeDir,
		Name:     "/",
	}
	bundleAML := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "bundle.yaml",
	}

	doneGot, errGot := checker.Process(root, nil)
	assert.NoError(t, errGot)
	assert.False(t, doneGot)

	doneGot, errGot = checker.Process(bundleAML, nil)
	assert.NoError(t, errGot)
	assert.False(t, doneGot)

	assert.False(t, checker.IsNestedBundleFormat())
}

func TestProcessWithNestedBundleFormat(t *testing.T) {
	checker := formatChecker{}

	// Simulate a flattened format as used by Operator Courier
	headers := []*tar.Header{
		&tar.Header{Name: "manifests", Typeflag: tar.TypeDir},
		&tar.Header{Name: "manifests/etcd", Typeflag: tar.TypeDir},
		&tar.Header{Name: "manifests/etcd/0.6.1", Typeflag: tar.TypeDir},
		&tar.Header{Name: "manifests/etcd/0.6.1/etcdcluster.crd.yaml", Typeflag: tar.TypeReg},
		&tar.Header{Name: "manifests/etcd/0.6.1/etcdoperator.clusterserviceversion.yaml", Typeflag: tar.TypeReg},
		&tar.Header{Name: "manifests/etcd/etcd.package.yaml", Typeflag: tar.TypeReg},
	}

	for _, header := range headers {
		doneGot, errGot := checker.Process(header, nil)
		assert.NoError(t, errGot)

		if checker.IsNestedBundleFormat() {
			assert.True(t, doneGot)
		}
	}

	assert.True(t, checker.IsNestedBundleFormat())
}
