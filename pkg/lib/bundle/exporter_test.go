package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/operator-framework/operator-registry/pkg/containertools"
)

func TestExportForBundleWithBadImage(t *testing.T) {
	exporter := NewExporterForBundle("foo", "", containertools.DockerTool)
	err := exporter.Export(true, false)
	assert.Error(t, err)

	err = exporter.Export(false, true)
	assert.Error(t, err)

	exporter = NewExporterForBundle("foo", "", containertools.NoneTool)
	err = exporter.Export(true, false)
	assert.Error(t, err)
}
