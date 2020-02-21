package bundle

import "testing"

func TestIsChartDir(t *testing.T) {
	validChartDir, err := IsChartDir("testdata/frobnitz")
	if !validChartDir {
		t.Errorf("unexpected error while reading chart-directory: (%v)", err)
		return
	}
	validChartDir, err = IsChartDir("testdata")
	if validChartDir || err == nil {
		t.Errorf("expected error but did not get any")
		return
	}
}
