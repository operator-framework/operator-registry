package bundle

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/operator-framework/operator-registry/pkg/containertools"
)

func TestExportForBundleWithBadImage(t *testing.T) {
	exporter := NewExporterForBundle("foo", "", containertools.DockerTool)
	err := exporter.Export(true, false)
	require.Error(t, err)

	err = exporter.Export(false, true)
	require.Error(t, err)

	exporter = NewExporterForBundle("foo", "", containertools.NoneTool)
	err = exporter.Export(true, false)
	require.Error(t, err)
}
