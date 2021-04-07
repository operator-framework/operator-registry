package action

import (
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/operator-framework/operator-registry/internal/declcfg"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddBundle(t *testing.T) {
	type spec struct {
		name               string
		configsDir         func() (string, error)
		bundle             BundleExtractor
		expectedConfigsDir string
		assertion          require.ErrorAssertionFunc
	}
	specs := []spec{
		{
			name:               "Success/NewBundle",
			configsDir:         func() (string, error) { return ioutil.TempDir("", "death-star") },
			expectedConfigsDir: "test-configs",
			bundle:             NewDirBundleExtractor("../../bundles/etcd.0.9.2"),
			assertion:          require.NoError,
		},
	}

	for _, s := range specs {
		t.Run(s.name, func(t *testing.T) {
			testDir, err := s.configsDir()
			require.NoError(t, err)
			defer func() {
				require.NoError(t, os.RemoveAll(testDir))
			}()
			request := AddConfigRequest{
				ConfigsDir: testDir,
				Bundles:    []BundleExtractor{s.bundle},
			}
			bundleAdder := NewBundleAdder(logrus.NewEntry(logrus.New()))
			err = bundleAdder.AddToConfig(request)
			s.assertion(t, err)

			expectedConfigs, err := declcfg.LoadDir(s.expectedConfigsDir)
			require.NoError(t, err)

			actualConfigs, err := declcfg.LoadDir(testDir)
			require.NoError(t, err)

			equalsDeclarativeConfig(t, expectedConfigs, actualConfigs)
		})
	}
}

func equalsDeclarativeConfig(t *testing.T, expected, actual *declcfg.DeclarativeConfig) {
	t.Helper()
	removeJSONWhitespace(expected)
	removeJSONWhitespace(actual)
	assert.ElementsMatch(t, expected.Packages, actual.Packages)
	assert.ElementsMatch(t, expected.Others, actual.Others)
	require.Equal(t, len(expected.Bundles), len(actual.Bundles))
	sort.SliceStable(expected.Bundles, func(i, j int) bool {
		return expected.Bundles[i].Name < expected.Bundles[j].Name
	})
	sort.SliceStable(actual.Bundles, func(i, j int) bool {
		return actual.Bundles[i].Name < actual.Bundles[j].Name
	})
	for i := range expected.Bundles {
		assert.ElementsMatch(t, expected.Bundles[i].Properties, actual.Bundles[i].Properties)
		expected.Bundles[i].Properties, actual.Bundles[i].Properties = nil, nil
		assert.Equal(t, expected.Bundles[i], actual.Bundles[i])
	}
	// In case new fields are added to the DeclarativeConfig struct in the future,
	// test that the rest is Equal.
	expected.Packages, actual.Packages = nil, nil
	expected.Bundles, actual.Bundles = nil, nil
	expected.Others, actual.Others = nil, nil
	assert.Equal(t, expected, actual)
}

func removeJSONWhitespace(cfg *declcfg.DeclarativeConfig) {
	replacer := strings.NewReplacer(" ", "", "\n", "")
	for ib := range cfg.Bundles {
		for ip := range cfg.Bundles[ib].Properties {
			cfg.Bundles[ib].Properties[ip].Value = []byte(replacer.Replace(string(cfg.Bundles[ib].Properties[ip].Value)))
		}
	}
	for io := range cfg.Others {
		for _, v := range cfg.Others[io].Blob {
			cfg.Others[io].Blob = []byte(replacer.Replace(string(v)))
		}
	}
}
