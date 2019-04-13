package appregistry

import (
	"github.com/operator-framework/operator-registry/pkg/apprclient"
	"github.com/sirupsen/logrus"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func NewManifestDecoder(logger *logrus.Entry, directory string) (*manifestDecoder, error) {
	bundle, err := NewBundleProcessor(logger, directory)
	if err != nil {
		return nil, err
	}

	return &manifestDecoder{
		logger:    logger,
		flattened: NewFlattenedProcessor(logger),
		nested:    bundle,
		walker:    &tarWalker{},
	}, nil
}

type result struct {
	// Flattened contains all flattened single file operator manifest(s).
	Flattened *RawOperatorManifestData

	// NestedDirectory points to the directory where all specified nested
	// operator bundle(s) have been written to.
	NestedDirectory string

	// FlattenedCount is the total number of flattened single-file operator
	// manifest(s) processed so far.
	FlattenedCount int

	// NestedCount is the total number of nested operator manifest(s)
	// processed so far.
	NestedCount int
}

// IsEmpty returns true if no operator manifest has been processed so far.
func (r *result) IsEmpty() bool {
	return r.FlattenedCount == 0 && r.NestedCount == 0
}

type manifestDecoder struct {
	logger    *logrus.Entry
	flattened *flattenedProcessor
	nested    *bundleProcessor
	walker    *tarWalker
}

// Decode iterates through each operator manifest blob that is encoded in a tar
// ball and processes it accordingly.
//
// On return, result.Flattened is populates with the set of operator manifest(s)
// specified in flattened format ( one file with data section). If there are no
// operator(s) in flattened format result.Flattened is set to nil
//
// On return, result.NestedDirectory is set to the path of the folder which
// contains operator manifest(s) specified in nested bundle format. If there are
// no operator(s) in nested bundle format then result.NestedCount  is set to
// zero.
//
// This function takes a best-effort approach. On return, err is set to an
// aggregated list of error(s) encountered. The caller should inspect the
// result object to determine the next steps.
func (d *manifestDecoder) Decode(manifests []*apprclient.OperatorMetadata) (result result, err error) {
	d.logger.Info("decoding the downloaded operator manifest(s)")

	getProcessor := func(isNested bool) (Processor, string) {
		if isNested {
			return d.nested, "nested"
		}

		return d.flattened, "flattened"
	}

	allErrors := []error{}
	for _, om := range manifests {
		loggerWithBlobID := d.logger.WithField("repository", om.RegistryMetadata.String())

		// Determine the format type of the manifest blob and select the right processor.
		checker := NewFormatChecker()
		walkError := d.walker.Walk(om.Blob, checker)
		if walkError != nil {
			loggerWithBlobID.Errorf("skipping, can't determine the format of the manifest - %v", walkError)
			allErrors = append(allErrors, err)
			continue
		}

		if checker.IsNestedBundleFormat() {
			result.NestedCount++
		}

		processor, format := getProcessor(checker.IsNestedBundleFormat())
		loggerWithBlobID.Infof("manifest format is - %s", format)

		walkError = d.walker.Walk(om.Blob, processor)
		if walkError != nil {
			loggerWithBlobID.Errorf("skipping due to error - %v", walkError)
			allErrors = append(allErrors, err)
			continue
		}

		loggerWithBlobID.Infof("decoded successfully")
	}

	result.NestedDirectory = d.nested.GetManifestDownloadDirectory()
	result.FlattenedCount = d.flattened.GetProcessedCount()

	// Merge all flattened operator manifest(s) into one.
	if d.flattened.GetProcessedCount() > 0 {
		d.logger.Info("merging all flattened manifests into a single configmap 'data' section")

		result.Flattened, err = d.flattened.MergeIntoDataSection()
		if err != nil {
			d.logger.Errorf("error merging flattened manifest(s) - %v", err)
			allErrors = append(allErrors, err)
		}
	}

	err = utilerrors.NewAggregate(allErrors)
	return
}
