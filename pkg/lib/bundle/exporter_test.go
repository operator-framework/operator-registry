package bundle

import (
	"testing"

	"github.com/operator-framework/operator-registry/pkg/containertools"
	"github.com/stretchr/testify/assert"
)

func TestExportForBundleWithBadImage(t *testing.T) {
	exporter := NewExporterForBundle("foo", "", containertools.DockerTool)
	err := exporter.Export(true)
	assert.Error(t, err)

	exporter = NewExporterForBundle("foo", "", containertools.NoneTool)
	err = exporter.Export(true)
	assert.Error(t, err)
}
