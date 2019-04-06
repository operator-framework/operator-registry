package appregistry

import (
	"archive/tar"
	"bytes"
	"io"

	"github.com/sirupsen/logrus"
)

func NewFlattenedProcessor(logger *logrus.Entry) *flattenedProcessor {
	return &flattenedProcessor{
		logger: logger,
		parser: &manifestYAMLParser{},
		merged: StructuredOperatorManifestData{},
	}
}

type flattenedProcessor struct {
	logger *logrus.Entry
	parser ManifestYAMLParser

	merged StructuredOperatorManifestData
	count  int
}

func (w *flattenedProcessor) GetProcessedCount() int {
	return w.count
}

// Process handles a flattened single file operator manifest.
//
// It expects a single file, as soon as the function encounters a file it parses
// the raw yaml and merges it into the uber manifest.
func (w *flattenedProcessor) Process(header *tar.Header, reader io.Reader) (done bool, err error) {
	if header.Typeflag != tar.TypeReg {
		return
	}

	// We ran into the first file, We don't need to walk the tar ball any
	// further. Instruct the tar walker to quit.
	defer func() {
		done = true
	}()

	writer := &bytes.Buffer{}
	if _, err = io.Copy(writer, reader); err != nil {
		return
	}

	rawYAML := writer.Bytes()
	manifest, err := w.parser.Unmarshal(rawYAML)
	if err != nil {
		return
	}

	w.merged.Packages = append(w.merged.Packages, manifest.Packages...)
	w.merged.CustomResourceDefinitions = append(w.merged.CustomResourceDefinitions, manifest.CustomResourceDefinitions...)
	w.merged.ClusterServiceVersions = append(w.merged.ClusterServiceVersions, manifest.ClusterServiceVersions...)

	w.count += 1
	return
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
func (w *flattenedProcessor) MergeIntoDataSection() (*RawOperatorManifestData, error) {
	manifests, err := w.parser.Marshal(&w.merged)
	if err != nil {
		return nil, err
	}

	return manifests, nil
}
